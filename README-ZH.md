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

due是一款基于Go语言开发的轻量级分布式游戏服务器框架。
其中，模块设计方面借鉴了[kratos](https://github.com/go-kratos/kratos)的模块设计思路，为开发者提供了较为灵活的集群构建方案。

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

### 4.说明

> 在due交流群中经常有小伙伴提及到Gate、Node、Mesh之间到底是个什么关系，这里就做一个统一的解答

* Gate：网关服，主要用于管理客户端连接，接收客户端的路由消息，并分发路由消息到不同的的Node节点服。
* Node:
  节点服，作为整个集群系统的核心组件，主要用于核心逻辑业务的编写。Node节点服务可以根据业务需要做成有状态或无状态的节点，当作为无状态的节点时，Node节点与Mesh微服务基本无异；但当Node节点作为有状态节点时，Node节点便不能随意更新进行重启操作。故而Node与Mesh分离的业务场景的价值就体现出来了。
* Mesh：微服务，主要用于无状态的业务逻辑编写。Mesh能做的功能Node一样可以完成，如何选择完全取决于自身业务场景，开发者可以根据自身业务场景灵活搭配。
* Master：管理服，主要用于GM后台管理功能的开发。

### 5.协议

在due框架中，通信协议统一采用size+header+route+seq+message的格式：

1.数据包

```
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
|                              size                             |h|   extcode   |             route             |              seq              |
+---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
|                                                                message data ...                                                               |
+-----------------------------------------------------------------------------------------------------------------------------------------------+
```

2.心跳包

```
 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+---------------------------------------------------------------+-+-------------+---------------------------------------------------------------+
|                              size                             |h|   extcode   |                      heartbeat time (ns)                      |
+---------------------------------------------------------------+-+-------------+---------------------------------------------------------------+
```

size: 4 bytes

- 包长度位
- 固定长度为4字节，且不可修改

header: 1 bytes

h: 1 bit

- 心跳标识位
- %x0 表示数据包
- %x1 表示心跳包

extcode: 7 bit

- 扩展操作码
- 暂未明确定义具体操作码

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

heartbeat time: 8 bytes

- 心跳数据
- 数据包无心跳数据
- 上行心跳包无需携带心跳数据，下行心跳包默认携带8 bytes的服务器时间（ns），可通过网络库配置进行设置是否携带下行包时间信息
- 此参数由网络框架层自动打包，服务端开发者不关注此参数，客户端开发者需关注此参数

### 6、相关工具链

1.安装protobuf编译器（使用场景：开发mesh微服务）

- Linux, using apt or apt-get, for example:

```shell
$ apt install -y protobuf-compiler
$ protoc --version  # Ensure compiler version is 3+
```

- MacOS, using Homebrew:

```shell
$ brew install protobuf
$ protoc --version  # Ensure compiler version is 3+
```

- Windows, download from [Github](https://github.com/protocolbuffers/protobuf/releases):

2.安装protobuf go代码生成工具（使用场景：开发mesh微服务）

```shell
go install github.com/gogo/protobuf/protoc-gen-gofast@latest
```

3.安装grpc代码生成工具（使用场景：使用[GRPC](https://grpc.io/)组件开发mesh微服务）

```shell
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

4.安装rpcx代码生成工具（使用场景：使用[RPCX](https://rpcx.io/)组件开发mesh微服务）

```shell
go install github.com/rpcxio/protoc-gen-rpcx@latest
```

5.安装gorm dao代码生成工具（使用场景：使用[GORM](https://gorm.io/)作为数据库orm）

```shell
go install github.com/dobyte/gorm-dao-generator@latest
```

6.安装mongo dao代码生成工具（使用场景：使用[MongoDB](https://github.com/mongodb/mongo-go-driver)作为数据库orm）

```shell
go install github.com/dobyte/mongo-dao-generator@latest
```

### 7.配置中心

1.功能介绍

配置中心主要定位于业务的配置管理，提供快捷灵活的配置方案。支持完善的读取、修改、删除、热更新等功能。

2.配置组件

* [file](config/file/README-ZH.md)
* [etcd](config/etcd/README-ZH.md)
* [consul](config/consul/README-ZH.md)

### 8.注册中心

1.功能介绍

注册中心用于集群实例的服务注册和发现。支撑整个集群的无感知停服、重启、动态扩容等功能。

2.相关组件

* [etcd](registry/etcd/README-ZH.md)
* [consul](registry/consul/README-ZH.md)

### 9.网络

### 10.快速开始

下面我们就通过两段简单的代码来体验一下due的魅力，Let's go~~

1.启动组件

```shell
docker-compose up
```

> docker-compose.yaml文件已在docker目录中备好，可以直接取用

2.获取框架

```shell
go get -u github.com/dobyte/due/v2@latest
go get -u github.com/dobyte/due/locate/redis/v2@latest
go get -u github.com/dobyte/due/network/ws/v2@latest
go get -u github.com/dobyte/due/registry/consul/v2@latest
go get -u github.com/dobyte/due/transport/rpcx/v2@latest
```

3.构建Gate服务器

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

4.构建Node服务器

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

5.构建测试客户端

```go
package main

import (
	"fmt"
	"github.com/dobyte/due/eventbus/nats/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
	"time"
)

const greet = 1

func main() {
	// 初始化事件总线
	eventbus.SetEventbus(nats.NewEventbus())
	// 创建容器
	container := due.NewContainer()
	// 创建客户端组件
	component := client.NewClient(
		client.WithClient(ws.NewClient()),
	)
	// 初始化监听
	initListen(component.Proxy())
	// 添加客户端组件
	container.Add(component)
	// 启动容器
	container.Serve()
}

// 初始化监听
func initListen(proxy *client.Proxy) {
	// 监听组件启动
	proxy.AddHookListener(cluster.Start, startHandler)
	// 监听连接建立
	proxy.AddEventListener(cluster.Connect, connectHandler)
	// 监听消息回复
	proxy.AddRouteHandler(greet, greetHandler)
}

// 组件启动处理器
func startHandler(proxy *client.Proxy) {
	if _, err := proxy.Dial(); err != nil {
		log.Errorf("gate connect failed: %v", err)
		return
	}
}

// 连接建立处理器
func connectHandler(conn *client.Conn) {
	doPushMessage(conn)
}

// 消息回复处理器
func greetHandler(ctx *client.Context) {
	res := &greetRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("invalid response message, err: %v", err)
		return
	}

	if res.Code != 0 {
		log.Errorf("node response failed, code: %d", res.Code)
		return
	}

	log.Info(res.Message)

	time.AfterFunc(time.Second, func() {
		doPushMessage(ctx.Conn())
	})
}

// 推送消息
func doPushMessage(conn *client.Conn) {
	err := conn.Push(&cluster.Message{
		Route: 1,
		Data: &greetReq{
			Message: fmt.Sprintf("I'm client, and the current time is: %s", xtime.Now().Format(xtime.DatetimeLayout)),
		},
	})
	if err != nil {
		log.Errorf("push message failed: %v", err)
	}
}

type greetReq struct {
	Message string `json:"message"`
}

type greetRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
```

### 11.支持组件

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

### 12.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)

### 13.其他客户端

[due-client-ts](https://github.com/dobyte/due-client-ts)

### 14.交流与讨论

<img title="" src="group_qrcode.jpeg" alt="交流群" width="175"><img title="" src="personal_qrcode.jpeg" alt="个人二维码" width="177">

个人微信：yuebanfuxiao