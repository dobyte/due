# due

##### [English Document](README.md)

[![Build Status](https://github.com/dobyte/due/workflows/Go/badge.svg)](https://github.com/dobyte/due/actions)
[![goproxy](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/dobyte/due.svg)](https://pkg.go.dev/github.com/dobyte/due)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/dobyte/due)](https://goreportcard.com/report/github.com/dobyte/due)
![Coverage](https://img.shields.io/badge/Coverage-17.4%25-red)
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

在due框架中，通信协议统一采用opcode+route+seq+message的格式：

1.Websocket数据包

```
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+-+-------------+-------------------------------+-------------------------------+
|h|   extcode   |             route             |              seq              |
+-+-------------+-------------------------------+-------------------------------+
|                                message data ...                               |
+-------------------------------------------------------------------------------+
```

2.Websocket心跳包

```
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+-+-------------+---------------------------------------------------------------+
|h|   extcode   |                       server time (ms)                        |
+-+-------------+---------------------------------------------------------------+
```

3.TCP数据包

```
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
|                              len                              |h|   extcode   |             route             |              seq              |
+---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
|                                                                message data ...                                                               |
+-----------------------------------------------------------------------------------------------------------------------------------------------+
```

4.TCP心跳包

```
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+---------------------------------------------------------------+-+-------------+---------------------------------------------------------------+
|                              len                              |h|   extcode   |                       server time (ms)                        |
+---------------------------------------------------------------+-+-------------+---------------------------------------------------------------+
```

len: 4 bytes

- TCP包长度位
- 固定长度为4字节，且不可修改
- 采用大端序
- WebSocket协议无包长度位
- 此参数由TCP网络框架自动打包生成，服务端开发者不关注此参数，客户端开发者需关注此参数

h: 1 bit

- 心跳标识位
- %x0 表示数据包
- %x1 表示心跳包
- 采用大端序
- 此参数由网络框架层自动打包生成，服务端开发者不关注此参数，客户端开发者需关注此参数

extcode: 7 bit

- 扩展操作码
- 暂未明确定义具体操作码
- 采用大端序
- 此参数由网络框架层自动打包生成，服务端开发者不关注此参数，客户端开发者需关注此参数

route: 1 bytes | 2 bytes | 4 bytes

- 消息路由
- 默认采用2字节，可通过打包器配置packet.routeBytes进行修改
- 不同的路由对应不同的业务处理流程
- 心跳包无消息路由位
- 此参数由业务打包器打包，服务器开发者和客户端开发者均要关心此参数

seq: 0 bytes | 1 bytes | 2 bytes | 4 bytes

- 消息序列号
- 默认采用2字节，可通过打包器配置packet.seqBytes进行修改
- 可通过将打包器配置packet.seqBytes设置为0来屏蔽使用序列号
- 消息序列号常用于请求/响应模型的消息对儿的确认
- 心跳包无消息序列号位
- 此参数由业务打包器packet.Packer打包，服务器开发者和客户端开发者均要关心此参数

message data: n bytes

- 消息数据
- 心跳包无消息数据
- 此参数由业务打包器packet.Packer打包，服务器开发者和客户端开发者均要关心此参数

server time: 8 bytes

- 心跳数据
- 数据包无心跳数据
- 上行心跳包无需携带心跳数据，下行心跳包默认携带8 bytes的服务器时间（ms），可通过网络库配置进行设置是否携带下行包时间信息
- 此参数由网络框架层自动打包，服务端开发者不关注此参数，客户端开发者需关注此参数

### 5.配置中心

1.功能介绍

配置中心主要定位于业务的配置管理，提供快捷灵活的配置方案。支持完善的读取、修改、删除、热更新等功能。

2.配置组件

* [file](config/file/README-ZH.md)
* [etcd](config/etcd/README-ZH.md)
* [consul](config/consul/README-ZH.md)

### 6.注册中心

1.功能介绍

注册中心用于集群实例的服务注册和发现。支撑整个集群的无感知停服、重启、动态扩容等功能。

2.相关组件

* [etcd](registry/etcd/README-ZH.md)
* [consul](registry/consul/README-ZH.md)

### 7.网络

### 6.快速开始

下面我们就通过两段简单的代码来体验一下due的魅力，Let's go~~

0.启动组件

```shell
docker-compose up
```

> docker-compose.yaml文件已在docker目录中备好，可以直接取用

1.获取框架

```shell
go get -u github.com/dobyte/due/v2@latest
go get -u github.com/dobyte/due/locate/redis/v2@latest
go get -u github.com/dobyte/due/network/ws/v2@latest
go get -u github.com/dobyte/due/registry/consul/v2@latest
go get -u github.com/dobyte/due/transport/rpcx/v2@latest
```

2.构建Gate服务器

```go
package main

import (
    "github.com/dobyte/due/locate/redis/v2"
    "github.com/dobyte/due/network/ws/v2"
    "github.com/dobyte/due/registry/consul/v2"
    "github.com/dobyte/due/transport/rpcx/v2"
    "github.com/dobyte/due/v2"
    "github.com/dobyte/due/v2/cluster/gate"
)

func main() {
    // 创建容器
    container := due.NewContainer()
    // 创建服务器
    server := ws.NewServer()
    // 创建用户定位器
    locator := redis.NewLocator()
    // 创建服务发现
    registry := consul.NewRegistry()
    // 创建RPC传输器
    transporter := rpcx.NewTransporter()
    // 创建网关组件
    component := gate.NewGate(
        gate.WithServer(server),
        gate.WithLocator(locator),
        gate.WithRegistry(registry),
        gate.WithTransporter(transporter),
    )
    // 添加网关组件
    container.Add(component)
    // 启动容器
    container.Serve()
}
```

3.构建Node服务器

```go
package main

import (
	"fmt"
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/registry/consul/v2"
	"github.com/dobyte/due/transport/rpcx/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"
)

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := consul.NewRegistry()
	// 创建RPC传输器
	transporter := rpcx.NewTransporter()
	// 创建网关组件
	component := node.NewNode(
		node.WithLocator(locator),
		node.WithRegistry(registry),
		node.WithTransporter(transporter),
	)
	// 注册路由
	component.Proxy().Router().AddRouteHandler(1, false, greetHandler)
	// 添加网关组件
	container.Add(component)
	// 启动容器
	container.Serve()
}

type greetReq struct {
	Name string `json:"name"`
}

type greetRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func greetHandler(ctx *node.Context) {
	req := &greetReq{}
	res := &greetRes{}
	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response message failed: %v", err)
		}
	}()

	if err := ctx.Request.Parse(req); err != nil {
		log.Errorf("parse request message failed: %v", err)
		res.Code = codes.InternalError.Code()
		return
	}

	res.Code = codes.OK.Code()
	res.Message = fmt.Sprintf("hello %s~~", req.Name)
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