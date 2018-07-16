GOPATH:=$(shell go env GOPATH)

.PHONY: proto test docker


proto:
	protoc --go_out=plugins=grpc:. pb/*.proto
	protoc -I. -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
      --grpc-gateway_out=logtostderr=true:. pb/*.proto
    protoc -I. -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
      --swagger_out=logtostderr=true:. pb/*.proto

build: proto

	go build -o zergkv zergkv.go

test:
	go test -v ./... -cover

docker:
    GOOS=linux GOARCH=amd64 go build -o zergkv zergkv.go
	docker build . -t zergwangj/zergkv:latest