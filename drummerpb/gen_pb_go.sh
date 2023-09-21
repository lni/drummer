#!/bin/bash

protoc --go_out=. --go-grpc_out=. --proto_path=../..:../vendor:. drummer.proto
mv github.com/lni/drummer/drummerpb/*.go ./
rm -rf github.com
