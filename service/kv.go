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
	"github.com/zergwangj/zergkv/raft"
	"context"
	"github.com/zergwangj/zergkv/pb"
	"google.golang.org/grpc"
)

type Kv struct {
	Raft 			*raft.Kv
	PeersGrpcMap	map[string]string
}

func NewKv(dir string, id string, addr string, peersGrpcMap map[string]string, peersRaftMap map[string]string) (*Kv, error) {
	raftKv, err := raft.NewKv(dir, id, addr, peersRaftMap)
	if err != nil {
		return nil, err
	}

	k := &Kv{
		Raft: 			raftKv,
		PeersGrpcMap: 	peersGrpcMap,
	}
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

	leaderLocal, leaderRaftAddr := k.Raft.GetLeader()
	if leaderLocal {
		err := k.Raft.Set(req.Key, req.Value)
		if err != nil {
			rsp.Error = err.Error()
			return rsp, nil
		}
		rsp.Error = ""
		return rsp, nil
	}

	leaderGrpcAddr := k.PeersGrpcMap[leaderRaftAddr]
	conn, err := grpc.Dial(leaderGrpcAddr, grpc.WithInsecure())
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

	leaderLocal, leaderRaftAddr := k.Raft.GetLeader()
	if leaderLocal {
		err := k.Raft.Delete(req.Key)
		if err != nil {
			rsp.Error = err.Error()
			return rsp, nil
		}
		rsp.Error = ""
		return rsp, nil
	}

	leaderGrpcAddr := k.PeersGrpcMap[leaderRaftAddr]
	conn, err := grpc.Dial(leaderGrpcAddr, grpc.WithInsecure())
	if err != nil {
		rsp.Error = err.Error()
		return rsp, nil
	}
	defer conn.Close()
	c := pb.NewKvClient(conn)
	return c.Delete(context.Background(), req)
}
