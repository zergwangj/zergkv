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

package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"github.com/zergwangj/zergkv/db"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	raftTimeOut = 10 * time.Second
)

type Kv struct {
	Dir   string
	Id    string
	Addr  string
	Peers map[string]string
	Raft  *raft.Raft
	Db    *db.Kv
}

type Command struct {
	Op  string
	Key []byte
	Val []byte
}

func NewKv(dir string, id string, addr string, peers map[string]string) (*Kv, error) {
	k := new(Kv)

	dbKv, err := db.NewKv(dir)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Faile to create db file: %s", err.Error()))
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(id)

	address, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Faile to resolve TCP address: %s", err.Error()))
	}
	transport, err := raft.NewTCPTransport(addr, address, 3, raftTimeOut, os.Stderr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create TCP transport: %s", err.Error()))
	}
	snapshot, err := raft.NewFileSnapshotStore(dir, 3, os.Stderr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create file snapshot store: %s", err.Error()))
	}
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dir, "raft.db"))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Faied to create log store and stable store: %s", err.Error()))
	}
	raftKv, err := raft.NewRaft(config, k, logStore, logStore, snapshot, transport)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create Raft system: %s", err.Error()))
	}

	log.Printf("[INFO] zergkv: Cluster local server(RAFT): %s (%s)", config.LocalID, transport.LocalAddr())
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			},
		},
	}
	for id, addr := range peers {
		serverId := raft.ServerID(id)
		serverAddr, _ := raft.NewInmemTransport(raft.ServerAddress(addr))
		log.Printf("[INFO] zergkv: Cluster peer server(RAFT): %s (%s)", serverId, serverAddr)
		configuration.Servers = append(configuration.Servers, raft.Server{
			ID:      serverId,
			Address: serverAddr,
		})
	}

	future := raftKv.BootstrapCluster(configuration)
	if future.Error() != nil {
		log.Printf("[DEBUG] zergkv: Boostrap cluster failed: %s", future.Error())
	}

	k.Dir = dir
	k.Id = id
	k.Addr = addr
	k.Raft = raftKv
	k.Db = dbKv
	return k, nil
}

func (k *Kv) Close() {
	k.Raft.Shutdown()
	k.Db.Close()
}

func (k *Kv) GetLeader() (bool, string) {
	return k.Raft.State() == raft.Leader, string(k.Raft.Leader())
}

func (k *Kv) Get(key []byte) ([]byte, error) {
	return k.Db.Get(key)
}

func (k *Kv) Set(key []byte, val []byte) error {
	if k.Raft.State() != raft.Leader {
		return errors.New("Not the leader, all changes to the system must go through the leader.")
	}

	c := &Command{
		Op:  "SET",
		Key: key,
		Val: val,
	}

	cmd, err := json.Marshal(c)
	if err != nil {
		return err
	}

	future := k.Raft.Apply(cmd, raftTimeOut)

	return future.Error()
}

func (k *Kv) Delete(key []byte) error {
	if k.Raft.State() != raft.Leader {
		return errors.New("Not the leader, all changes to the system must go through the leader.")
	}

	c := &Command{
		Op:  "DELETE",
		Key: key,
	}

	cmd, err := json.Marshal(c)
	if err != nil {
		return err
	}

	future := k.Raft.Apply(cmd, raftTimeOut)

	return future.Error()
}

func (k *Kv) Join(nodeId string, nodeAddr string) error {
	if k.Raft.State() != raft.Leader {
		return errors.New("Not the leader, all changes to the system must go through the leader.")
	}

	serverId := raft.ServerID(nodeId)
	serverAddr := raft.ServerAddress(nodeAddr)

	log.Println("Received a join request: NodeId: %s, NodeAddr: %s", serverId, serverAddr)

	// AddVoter will add the given server to the cluster as a staging server.
	// If the server is already in the cluster as a voter, this does nothing.
	// The leader will promote the staging server to a voter once that server is ready.
	future := k.Raft.AddVoter(serverId, serverAddr, 0, 0)
	if future.Error() != nil {
		return future.Error()
	}

	log.Printf("The node [NodeId: %s, NodeAddr: %s] has successfully joined the cluster", serverId, serverAddr)

	return nil
}

////////////////////////////// FSM /////////////////////////////////
// FSM provides an interface that can be implementated by clients //
// to make use of replicated log.                                 //
////////////////////////////// FSM /////////////////////////////////

// Apply log is invoked once a log enetry is committed.
func (k *Kv) Apply(log *raft.Log) interface{} {
	var c Command
	if err := json.Unmarshal(log.Data, &c); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal command: %s", err.Error()))
	}

	switch c.Op {
	case "SET":
		return k.applySet(c.Key, c.Val)
	case "DELETE":
		return k.applyDelete(c.Key)
	default:
		return fmt.Sprintf("Unsupported operation: %s", c.Op)
	}

	return nil
}

// Snapshot is used to support log compaction. This call should return
// an FSMSnapshot which can be used to save a point-in-time snapshot of the FSM.
func (k *Kv) Snapshot() (raft.FSMSnapshot, error) {
	// TODO
	return nil, nil
}

// Restore is used to restore an FSM from a snapshot.
func (k *Kv) Restore(rc io.ReadCloser) error {
	// TODO
	return nil
}

func (k *Kv) applySet(key []byte, val []byte) interface{} {
	err := k.Db.Set(key, val)
	if err != nil {
		return err.Error()
	}
	return nil
}

func (k *Kv) applyDelete(key []byte) interface{} {
	err := k.Db.Delete(key)
	if err != nil {
		return err.Error()
	}
	return nil
}
