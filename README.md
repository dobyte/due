# due

[![Build Status](https://github.com/dobyte/due/workflows/Go/badge.svg)](https://github.com/dobyte/due/actions)
[![goproxy](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/dobyte/due.svg)](https://pkg.go.dev/github.com/dobyte/due)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

### 1.介绍

due是一款基于Go语言开发的轻量级分布式游戏服务器框架。 其中，模块设计方面借鉴了[kratos](https://github.com/go-kratos/kratos)的模块设计思路，为开发者提供了较为灵活的集群构建方案。

![架构图](architecture.jpg)

### 2.优势

* 简单性：架构简单，源码简洁易理解。
* 便捷性：仅暴露必要的调用接口，减轻开发者的心智负担。
* 高效性：框架原生提供tcp、kcp、ws等协议的服务器，方便开发者快速构建各种类型的网关服务器。
* 扩展性：采用良好的接口设计，方便开发者设计实现自有功能。
* 平滑性：引入信号量，通过控制服务注册中心来实现优雅地重启。
* 扩容性：通过优雅的路由分发机制，理论上可实现无限扩容。
* 易调试：框架原生提供了tcp、kcp、ws等协议的客户端，方便开发者进行独立的调试全流程调试。

### 3.功能

* 网关：支持tcp、kcp、ws等协议的网关服务器。
* 日志：支持std、zap、logrus、aliyun、tencent等多种日志组件。
* 注册：支持consul、etcd、k8s、nacos、servicecomb、zookeeper等多种服务注册中心。
* 协议：支持json、protobuf等多种通信协议。
* 配置：支持json、yaml、toml、xml、ini等多种文件格式。
* 通信：支持grpc、rpcx等多种高性能传输方案。
* 重启：支持服务器的平滑重启。

### 4.协议

在due框架中，通信协议统一采用route+message的格式：

```
-------------------
| route | message |
-------------------
```

tcp协议格式：

```
-------------------------
| len | route | message |
-------------------------
```

说明：

1. route表示消息路由，固定为4字节，不同的路由对应不同的业务处理流程。
2. message表示消息体，采用json或protobuf编码。
3. 默认使用小端序编码。
4. 选择使用tcp协议时，为了解决粘包问题，还应在包前面加上包长度len，固定为4字节，默认使用小端序编码。

### 5.快速开始

下面我们就通过两段简单的代码来体验一下due的魅力，Let's go~~

0.启动组件
```shell
docker-compose up
```
> docker-compose.yaml文件已在docker目录中备好，可以直接取用

1.获取框架

```shell
go get github.com/dobyte/due@latest
go get github.com/dobyte/due/network/ws@latest
go get github.com/dobyte/due/registry/consul@latest
```

2.构建Gate服务器

```go
package main

import (
	"github.com/dobyte/due"
	"github.com/dobyte/due/cluster/gate"
	"github.com/dobyte/due/network/ws"
	"github.com/dobyte/due/registry/consul"
	"github.com/dobyte/due/third/redis"
	"github.com/dobyte/due/transport/grpc"
)

func main() {
	// 创建容器
	container := due.NewContainer()

	// 创建服务器
	svr := ws.NewServer(
		ws.WithServerListenAddr(":3553"),
		ws.WithServerMaxConnNum(5000),
	)

	// 创建redis客户端
	rds := redis.NewClient(
		redis.WithAddrs("127.0.0.1:6379"),
		redis.WithDB(0),
	)

	// 创建grpc服务器
	gs := grpc.NewServer(
		grpc.WithServerListenAddr(":8081"),
	)

	// 创建registry注册发现
	registry := consul.NewRegistry(
		consul.WithAddress("127.0.0.1:8500"),
	)

	// 创建网关
	server := gate.NewGate(
		gate.WithRedis(rds),
		gate.WithServer(svr),
		gate.WithGRPCServer(gs),
		gate.WithRegistry(registry),
	)

	// 添加组件
	container.Add(server)
	// 启动服务器
	container.Serve()
}
```

2.构建Node服务器

```go
package main

import (
	"github.com/dobyte/due"
	"github.com/dobyte/due/cluster/node"
	"github.com/dobyte/due/registry/consul"
	"github.com/dobyte/due/third/redis"
	"github.com/dobyte/due/transport/grpc"
)

func main() {
	// 创建容器
	container := due.NewContainer()

	// 创建redis客户端
	rds := redis.NewClient(
		redis.WithAddrs("127.0.0.1:6379"),
		redis.WithDB(0),
	)

	// 创建grpc服务器
	gs := grpc.NewServer(
		grpc.WithServerListenAddr(":8082"),
	)

	// 创建registry注册发现
	registry := consul.NewRegistry(
		consul.WithAddress("127.0.0.1:8500"),
	)

	// 创建节点服务器
	server := node.NewNode(
		node.WithRedis(rds),
		node.WithGRPCServer(gs),
		node.WithRegistry(registry),
	)

	// 注册路由
	server.Proxy().AddRoute(1, false, greetHandler)
	// 添加组件
	container.Add(server)
	// 启动服务器
	container.Serve()
}

func greetHandler(req node.Request) {
	_ = req.Response([]byte("hello world~~"))
}
```

3.构建客户端
```go
package main

import (
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network"
	"github.com/dobyte/due/network/ws"
	"github.com/dobyte/due/packet"
)

var (
	handlers map[int32]handlerFunc
)

type handlerFunc func(conn network.Conn, buffer []byte)

func init() {
	handlers = map[int32]handlerFunc{
		1: greetHandler,
	}
}

func main() {
	client := ws.NewClient(ws.WithClientDialUrl("ws://127.0.0.1:3553"))

	client.OnConnect(func(conn network.Conn) {
		log.Infof("connection is opened")
	})
	client.OnDisconnect(func(conn network.Conn) {
		log.Infof("connection is closed")
	})
	client.OnReceive(func(conn network.Conn, msg []byte, msgType int) {
		message, err := packet.Unpack(msg)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		handler, ok := handlers[message.Route]
		if !ok {
			log.Errorf("the route handler is not registered, route:%v", message.Route)
			return
		}
		handler(conn, message.Buffer)
	})

	conn, err := client.Dial()
	if err != nil {
		log.Fatalf("dial failed: %v", err)
	}

	if err = push(conn, 1, []byte("hello due~~")); err != nil {
		log.Errorf("push message failed: %v", err)
	}

	select {}
}

func greetHandler(conn network.Conn, buffer []byte) {
	log.Infof("received message from server: %s", string(buffer))
}

func push(conn network.Conn, route int32, buffer []byte) error {
	msg, err := packet.Pack(&packet.Message{
		Route:  route,
		Buffer: buffer,
	})
	if err != nil {
		return err
	}

	return conn.Push(msg)
}
```

### 6.详细示例

更多详细示例请点击[example](example/)