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
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net"
	"log"
	"google.golang.org/grpc"
	"github.com/zergwangj/zergkv/service"
	"github.com/zergwangj/zergkv/pb"
	"strings"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "zergkv",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		id, _ := cmd.Flags().GetString("id")
		host, _ := cmd.Flags().GetString("host")
		grpcPort, _ := cmd.Flags().GetString("grpc_port")
		raftPort, _ := cmd.Flags().GetString("raft_port")
		peers, _ := cmd.Flags().GetStringSlice("peers")

		lis, err := net.Listen("tcp", net.JoinHostPort(host, grpcPort))
		if err != nil {
			log.Fatal("Failed to listen: %v", err)
		}
		s := grpc.NewServer()
		peersGrpcMap := make(map[string]string)
		peersRaftMap := make(map[string]string)
		for _, peer := range peers {
			strs := strings.Split(peer, ":")
			peerId, peerHost, peerGrpcPort, peerRaftPort := strs[0], strs[1], strs[2], strs[3]
			peerIps, _ := net.LookupHost(peerHost)
			for _, peerIp := range peerIps {
				peersGrpcMap[net.JoinHostPort(peerIp, peerRaftPort)] = net.JoinHostPort(peerIp, peerGrpcPort)
			}
			peersRaftMap[peerId] = net.JoinHostPort(peerHost, peerRaftPort)
		}
		srv, err := service.NewKv(dir, id, net.JoinHostPort(host, raftPort), peersGrpcMap, peersRaftMap)
		if err != nil {
			log.Fatal("Failed to create kv serivce")
		}
		pb.RegisterKvServer(s, srv)
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
	rootCmd.Flags().String("dir", "", "Work directory")
	rootCmd.Flags().String("id", "zkv", "Server identifier")
	rootCmd.Flags().String("host", "", "Server listen host")
	rootCmd.Flags().String("grpc_port", "5354", "Grpc port")
	rootCmd.Flags().String("raft_port", "5355", "Raft port")
	rootCmd.Flags().StringSlice("peers", nil, "Cluster peer servers")
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
