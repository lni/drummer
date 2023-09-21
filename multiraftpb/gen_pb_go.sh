#!/bin/bash

protoc --go_out=. --go-grpc_out=. --proto_path=../..:../vendor:. multiraft.proto
mv github.com/lni/drummer/multiraftpb/*.go ./
rm -rf github.com
