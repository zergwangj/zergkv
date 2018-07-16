// Copyright © 2018 zergwangj zergwangj@163.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zergwangj/zergkv/pb"
	"github.com/zergwangj/zergkv/service"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"strings"
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"net/http"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "zergkv",
	Short: "Distributed key value database in Go and Raft",
	Long:  `Distributed key value database in Go and Raft`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		serverName, _ := cmd.Flags().GetString("server_name")
		workDir, _ := cmd.Flags().GetString("work_dir")
		restfulAddr, _ := cmd.Flags().GetString("restful_addr")
		advertiseAddr, _ := cmd.Flags().GetString("advertise_addr")
		clusterAddr, _ := cmd.Flags().GetString("cluster_addr")
		peers, _ := cmd.Flags().GetStringSlice("peers")

		log.Println("[INFO] zergkv: ==> Starting zergkv server...")
		log.Printf("[INFO] zergkv: Server name: %s", serverName)
		log.Printf("[INFO] zergkv: Work directory: %s", workDir)
		log.Printf("[INFO] zergkv: Restful address(HTTP): %s", restfulAddr)
		log.Printf("[INFO] zergkv: Advertise address(GRPC): %s", advertiseAddr)
		log.Printf("[INFO] zergkv: Cluster address(RAFT): %s", clusterAddr)

		lis, err := net.Listen("tcp", advertiseAddr)
		if err != nil {
			log.Fatalf("[ERROR] zergkv: Failed to listen: %v", err)
		}
		s := grpc.NewServer()

		peersRaft2GrpcMap := make(map[string]string)
		peersRaftMap := make(map[string]string)
		log.Println("[INFO] zergkv: Peers: ")
		for _, peer := range peers {
			strs := strings.Split(peer, ":")
			peerName, peerHost, peerGrpcPort, peerRaftPort := strs[0], strs[1], strs[2], strs[3]
			log.Printf("[INFO] zergkv: ------ %s (HOST: %s GRPC: %s, RAFT: %s)", peerName, peerHost, peerGrpcPort, peerRaftPort)
			grpcAddr := net.JoinHostPort(peerHost, peerGrpcPort)
			raftAddr := net.JoinHostPort(peerHost, peerRaftPort)
			peersRaft2GrpcMap[raftAddr] = grpcAddr
			peersRaftMap[peerName] = raftAddr
		}
		srv, err := service.NewKv(workDir, serverName, clusterAddr, peersRaftMap, peersRaft2GrpcMap)
		if err != nil {
			log.Fatalf("[ERROR] zergkv: Failed to create kv service: %s", err.Error())
		}
		defer srv.Close()
		pb.RegisterKvServer(s, srv)

		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithInsecure()}
		err = pb.RegisterKvHandlerFromEndpoint(ctx, mux, advertiseAddr, opts)
		if err != nil {
			log.Fatalf("[ERROR] zergkv: Failed to create kv gateway: %s", err.Error())
		}

		go http.ListenAndServe(restfulAddr, mux)
		s.Serve(lis)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.zergkv.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().String("server_name", "", "Server name")
	rootCmd.Flags().String("work_dir", "", "Work directory")
	rootCmd.Flags().String("restful_addr", "", "Restful address")
	rootCmd.Flags().String("advertise_addr", "", "Advertise address")
	rootCmd.Flags().String("cluster_addr", "", "Cluster address")
	rootCmd.Flags().StringSlice("peers", nil, "Peers")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".zergkv" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".zergkv")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
