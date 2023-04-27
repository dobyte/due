# due
[![Build Status](https://github.com/dobyte/due/workflows/Go/badge.svg)](https://github.com/dobyte/due/actions)
[![goproxy](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/dobyte/due.svg)](https://pkg.go.dev/github.com/dobyte/due)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/dobyte/due)](https://goreportcard.com/report/github.com/dobyte/due)
![Coverage](https://img.shields.io/badge/Coverage-17.8%25-red)
[![Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

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
* 可管理：提供完善的后台管理接口，方便开发者快速实现自定义的后台管理功能。

### 3.功能

* 网关：支持tcp、kcp、ws等协议的网关服务器。
* 日志：支持std、zap、logrus、aliyun、tencent等多种日志组件。
* 注册：支持consul、etcd、k8s、nacos、servicecomb、zookeeper等多种服务注册中心。
* 协议：支持json、protobuf（gogo/protobuf）、msgpack等多种通信协议。
* 配置：支持json、yaml、toml、xml等多种文件格式。
* 通信：支持grpc、rpcx等多种高性能通信方案。
* 重启：支持服务器的平滑重启。
* 事件：支持redis、nats、kafka、rabbitMQ等事件总线实现方案。
* 加密：支持rsa、ecc等多种加密方案。
* 服务：支持grpc、rpcx等多种微服务解决方案。
* 灵活：支持单体、分布式等多种架构方案。
* 管理：提供master后台管理服相关接口支持。

> 注：出于性能考虑，protobuf协议默认使用[gogo/protobuf](https://github.com/gogo/protobuf)进行编解码，在生成go代码时请使用gogo库的protoc-gen-xxxx。

```bash
go install github.com/gogo/protobuf/protoc-gen-gofast@latest
```

### 4.协议

在due框架中，通信协议统一采用route+message的格式：

```
-------------------------
| seq | route | message |
-------------------------
```

tcp协议格式：

```
-------------------------------
| len | seq | route | message |
-------------------------------
```

说明：

1. seq表示请求序列号，默认为2字节，常用于请求、响应对的确认。可通过配置文件修改
2. route表示消息路由，默认为2字节，不同的路由对应不同的业务处理流程。可通过配置文件修改
3. message表示消息体，采用json或protobuf编码。
4. 打包器，默认使用小端序编码。可通过配置文件修改
5. 选择使用tcp协议时，为了解决粘包问题，还应在包前面加上包长度len，固定为4字节，默认使用小端序编码。

### 5.心跳

很意外，在due框架中，我们并没有采用0号路由来作为默认的心跳包来检测，默认我们采用的空包作为心跳检测包。

> 设计初衷：不想心跳检测侵入到业务路由层，哪怕是特殊的0号路由。

ws心跳包格式：

```go
[]byte
```

tcp心跳包格式：

```
-------
| len |
-------
|  0  |
-------
```

说明：

1. ws协议心跳包默认为空bytes。
2. 选择使用tcp协议时，为了解决粘包问题，还应在包前面加上包长度len，固定为4字节，包长度固定为0。

### 6.快速开始

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
go get github.com/dobyte/due/registry/etcd@latest
go get github.com/dobyte/due/locate/redis@latest
go get github.com/dobyte/due/transport/grpc@latest
```

2.构建Gate服务器

```go
package main

import (
   "github.com/dobyte/due"
   cluster "github.com/dobyte/due/cluster/gate"
   "github.com/dobyte/due/locate/redis"
   "github.com/dobyte/due/network/ws"
   "github.com/dobyte/due/registry/etcd"
   "github.com/dobyte/due/transport/grpc"
)

func main() {
   // 创建容器
   container := due.NewContainer()
   // 创建网关组件
   gate := cluster.NewGate(
      cluster.WithServer(ws.NewServer()),
      cluster.WithLocator(redis.NewLocator()),
      cluster.WithRegistry(etcd.NewRegistry()),
      cluster.WithTransporter(grpc.NewTransporter()),
   )
   // 添加网关组件
   container.Add(gate)
   // 启动容器
   container.Serve()
}

```

3.构建Node服务器

```go
package main

import (
   "github.com/dobyte/due"
   cluster "github.com/dobyte/due/cluster/node"
   "github.com/dobyte/due/locate/redis"
   "github.com/dobyte/due/registry/etcd"
   "github.com/dobyte/due/transport/grpc"
)

func main() {
   // 创建容器
   container := due.NewContainer()
   // 创建节点组件
   node := cluster.NewNode(
      cluster.WithLocator(redis.NewLocator()),
      cluster.WithRegistry(etcd.NewRegistry()),
      cluster.WithTransporter(grpc.NewTransporter()),
   )
   // 注册路由
   node.Proxy().Router().AddRouteHandler(1, false, greetHandler)
   // 添加组件
   container.Add(node)
   // 启动服务器
   container.Serve()
}

func greetHandler(ctx *cluster.Context) {
   _ = ctx.Response([]byte("hello world~~"))
}

```

4.构建Mesh服务

```go
package main

import (
   "context"
   "github.com/dobyte/due"
   cluster "github.com/dobyte/due/cluster/mesh"
   "github.com/dobyte/due/locate/redis"
   "github.com/dobyte/due/log"
   "github.com/dobyte/due/mode"
   "github.com/dobyte/due/registry/consul"
   "github.com/dobyte/due/transport/rpcx"
)

func main() {
   // 开启调试模式
   mode.SetMode(mode.DebugMode)
   // 创建容器
   container := due.NewContainer()
   // 创建网格组件
   mesh := cluster.NewMesh(
      cluster.WithLocator(redis.NewLocator()),
      cluster.WithRegistry(consul.NewRegistry()),
      cluster.WithTransporter(rpcx.NewTransporter()),
   )
   // 初始化业务
   NewWalletService(mesh.Proxy()).Init()
   // 添加网格组件
   container.Add(mesh)
   // 启动容器
   container.Serve()
}

// WalletService 钱包服务
type WalletService struct {
   proxy *cluster.Proxy
}

type IncrGoldRequest struct {
   UID  int64
   Gold int64
}

type IncrGoldReply struct {
}

func NewWalletService(proxy *cluster.Proxy) *WalletService {
   return &WalletService{proxy: proxy}
}

func (w *WalletService) Init() {
   w.proxy.AddServiceProvider("wallet", "Wallet", w)
}

func (w *WalletService) IncrGold(ctx context.Context, req *IncrGoldRequest, reply *IncrGoldReply) error {
   log.Infof("incr %d gold success for uid: %d", req.Gold, req.UID)

   return nil
}

```


5.构建测试客户端

```go
package main

import (
    "github.com/dobyte/due/config"
    "github.com/dobyte/due/log"
    "github.com/dobyte/due/mode"
    "github.com/dobyte/due/network"
    "github.com/dobyte/due/network/ws"
    "github.com/dobyte/due/packet"
)

var handlers map[int32]handlerFunc

type handlerFunc func(conn network.Conn, buffer []byte)

func init() {
    handlers = map[int32]handlerFunc{
        1: greetHandler,
    }
}

func main() {
    // 创建客户端
    client := ws.NewClient()
    // 监听连接
    client.OnConnect(func(conn network.Conn) {
        log.Infof("connection is opened")
    })
    // 监听断开连接
    client.OnDisconnect(func(conn network.Conn) {
        log.Infof("connection is closed")
    })
    // 监听收到消息
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
        Seq:    1,
        Route:  route,
        Buffer: buffer,
    })
    if err != nil {
        return err
    }

    return conn.Push(msg)
}
```

### 7.支持组件

1. 日志组件
   * zap: github.com/dobyte/due/log/zap
   * logrus: github.com/dobyte/due/log/logrus
   * aliyun: github.com/dobyte/due/log/aliyun
   * tencent: github.com/dobyte/due/log/zap
2. 网络组件
   * ws: github.com/dobyte/due/network/ws
   * tcp: github.com/dobyte/due/network/tcp
3. 注册发现
   * etcd: github.com/dobyte/due/registry/etcd
   * consul: github.com/dobyte/due/registry/consul
4. 传输组件
   * grpc: github.com/dobyte/due/transporter/grpc
   * rpcx: github.com/dobyte/due/transporter/rpcx
5. 定位组件
   * redis: github.com/dobyte/due/locate/redis
6. 事件总线
   * redis: github.com/dobyte/due/eventbus/redis
   * nats: github.com/dobyte/due/eventbus/nats
   * kafka: github.com/dobyte/due/eventbus/kafka

### 8.详细示例

更多详细示例请点击[due-example](https://github.com/dobyte/due-example)

### 9.其他客户端

[due-client-ts](https://github.com/dobyte/due-client-ts)

### 10.交流与讨论

<img title="" src="group_qrcode.jpeg" alt="交流群" width="175"><img title="" src="personal_qrcode.jpeg" alt="个人二维码" width="177">

个人微信：yuebanfuxiao