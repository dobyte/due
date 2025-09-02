#!/bin/bash

readonly directory=$(cd "$(dirname "$0")" && pwd)
readonly modules=(
	"./"
	"./cache/redis"
    "./cache/memcache"
	"./component/http"
    "./config/consul"
    "./config/etcd"
    "./config/nacos"
    "./crypto/rsa"
    "./crypto/ecc"
    "./eventbus/kafka"
    "./eventbus/nats"
    "./eventbus/redis"
    "./locate/redis"
    "./lock/redis"
    "./lock/memcache"
    "./log/aliyun"
    "./log/tencent"
    "./network/kcp"
    "./network/tcp"
    "./network/ws"
    "./registry/consul"
    "./registry/etcd"
    "./registry/nacos"
    "./transport/rpcx"
    "./transport/grpc"
)

for module in ${modules[@]}
do
  cd "${module}"
  go mod tidy
  cd "${directory}"
done