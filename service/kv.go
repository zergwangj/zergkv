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

package service

import (
	"context"
	"github.com/zergwangj/zergkv/pb"
	"github.com/zergwangj/zergkv/raft"
	"google.golang.org/grpc"
	"log"
	"net"
	"time"
)

const (
	updateLeaderGrpcAddrTimeOut = 1 * time.Second
)

type Kv struct {
	Raft              *raft.Kv
	PeersRaft2GrpcMap map[string]string
	LeaderGrpcAddr    string
}

func NewKv(dir string, id string, addr string, peersRaftMap map[string]string, peersRaft2GrpcMap map[string]string) (*Kv, error) {
	raftKv, err := raft.NewKv(dir, id, addr, peersRaftMap)
	if err != nil {
		return nil, err
	}

	k := &Kv{
		Raft:              raftKv,
		PeersRaft2GrpcMap: peersRaft2GrpcMap,
	}

	go k.UpdateLeaderGrpcAddr()

	return k, nil
}

func (k *Kv) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	rsp := new(pb.GetResponse)

	value, err := k.Raft.Get(req.Key)
	if err != nil {
		rsp.Error = err.Error()
		return rsp, nil
	}
	rsp.Value = value
	return rsp, nil
}

func (k *Kv) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	rsp := new(pb.SetResponse)

	isLeaderLocal, _ := k.Raft.GetLeader()
	if isLeaderLocal {
		err := k.Raft.Set(req.Key, req.Value)
		if err != nil {
			rsp.Error = err.Error()
			return rsp, nil
		}
		rsp.Error = ""
		return rsp, nil
	}

	conn, err := grpc.Dial(k.LeaderGrpcAddr, grpc.WithInsecure())
	if err != nil {
		rsp.Error = err.Error()
		return rsp, nil
	}
	defer conn.Close()
	c := pb.NewKvClient(conn)
	return c.Set(context.Background(), req)
}

func (k *Kv) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	rsp := new(pb.DeleteResponse)

	isLeaderLocal, _ := k.Raft.GetLeader()
	if isLeaderLocal {
		err := k.Raft.Delete(req.Key)
		if err != nil {
			rsp.Error = err.Error()
			return rsp, nil
		}
		rsp.Error = ""
		return rsp, nil
	}

	conn, err := grpc.Dial(k.LeaderGrpcAddr, grpc.WithInsecure())
	if err != nil {
		rsp.Error = err.Error()
		return rsp, nil
	}
	defer conn.Close()
	c := pb.NewKvClient(conn)
	return c.Delete(context.Background(), req)
}

func (k *Kv) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	rsp := new(pb.JoinResponse)

	err := k.Raft.Join(req.NodeId, req.NodeAddr)
	if err != nil {
		rsp.Error = err.Error()
		return rsp, nil
	}
	rsp.Error = ""
	return rsp, nil
}

func (k *Kv) UpdateLeaderGrpcAddr() {
	go func() {
		for {
			timer := time.NewTicker(updateLeaderGrpcAddrTimeOut)

			select {
			case <-timer.C:
				isLeaderLocal, leaderRaftAddr := k.Raft.GetLeader()
				if !isLeaderLocal {
					for peerRaftAddr, peerGrpcAddr := range k.PeersRaft2GrpcMap {
						if peerRaftAddr == leaderRaftAddr {
							if k.LeaderGrpcAddr != peerGrpcAddr {
								// TODO add lock?
								k.LeaderGrpcAddr = peerGrpcAddr
								log.Printf("[INFO] zergkv: New leader advertise address: %s", k.LeaderGrpcAddr)
							}
							return
						}

						peerRaftHost, peerRaftPort, _ := net.SplitHostPort(peerRaftAddr)
						peerRaftIPs, err := net.LookupIP(peerRaftHost)
						if err != nil {
							return
						}
						for _, peerRaftIP := range peerRaftIPs {
							newPeerRaftAddr := net.JoinHostPort(peerRaftIP.String(), peerRaftPort)
							if newPeerRaftAddr == leaderRaftAddr {
								if k.LeaderGrpcAddr != peerGrpcAddr {
									// TODO add lock?
									k.LeaderGrpcAddr = peerGrpcAddr
									log.Printf("[INFO] zergkv: New leader advertise address: %s", k.LeaderGrpcAddr)
								}
								return
							}
						}
					}
				}
			}
		}
	}()
}
