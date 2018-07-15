package api

import (
	"context"
	"errors"
	"github.com/zergwangj/zergkv/pb"
	"google.golang.org/grpc"
)

type Kv struct {
	Conn *grpc.ClientConn
}

func NewKv(advertiseAddr string) (*Kv, error) {
	conn, err := grpc.Dial(advertiseAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	k := &Kv{
		Conn: conn,
	}

	return k, nil
}

func (k *Kv) Close() {
	k.Conn.Close()
}

func (k *Kv) Get(key []byte) ([]byte, error) {
	client := pb.NewKvClient(k.Conn)
	rsp, err := client.Get(context.Background(), &pb.GetRequest{
		Key: key,
	})
	if err != nil {
		return nil, err
	}
	if len(rsp.Error) > 0 {
		return nil, errors.New(rsp.Error)
	}
	return rsp.Value, nil
}

func (k *Kv) Set(key []byte, value []byte) error {
	client := pb.NewKvClient(k.Conn)
	rsp, err := client.Set(context.Background(), &pb.SetRequest{
		Key:   key,
		Value: value,
	})
	if err != nil {
		return err
	}
	if len(rsp.Error) > 0 {
		return errors.New(rsp.Error)
	}
	return nil
}

func (k *Kv) Delete(key []byte) error {
	client := pb.NewKvClient(k.Conn)
	rsp, err := client.Delete(context.Background(), &pb.DeleteRequest{
		Key: key,
	})
	if err != nil {
		return err
	}
	if len(rsp.Error) > 0 {
		return errors.New(rsp.Error)
	}
	return nil
}
