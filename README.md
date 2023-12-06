# due

##### [中文文档](README-ZH.md)

[![Build Status](https://github.com/dobyte/due/workflows/Go/badge.svg)](https://github.com/dobyte/due/actions)
[![goproxy](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)](https://goproxy.cn/stats/github.com/dobyte/due/badges/download-count.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/dobyte/due.svg)](https://pkg.go.dev/github.com/dobyte/due)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/dobyte/due)](https://goreportcard.com/report/github.com/dobyte/due)
![Coverage](https://img.shields.io/badge/Coverage-17.4%25-red)
[![Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

### 1.Introduction

due is a lightweight distributed game server framework developed based on Go language. Among them, the module design draws on the module design ideas of [kratos](https://github.com/go-kratos/kratos) to provide developers with a more flexible cluster construction solution.

![architecture](architecture.jpg)

### 2.Advantages

* Simplicity: The architecture is simple, and the source code is concise and easy to understand.
* Convenience: Only the necessary calling interfaces are exposed, reducing the mental burden on developers.
* Efficiency: The framework natively provides servers for protocols such as tcp, kcp, and ws, making it easy for developers to quickly build various types of gateway servers.
* Scalability: Use good interface design to facilitate developers to design and implement their own functions.
* Smoothness: Introduce semaphores to achieve graceful restart by controlling the service registration center.
* Scalability: Through the elegant route distribution mechanism, unlimited expansion can be theoretically achieved.
* Easy debugging: The framework natively provides clients for protocols such as tcp, kcp, and ws, which facilitates developers to conduct independent debugging of the entire process.
* Manageable: Provides a complete backend management interface to facilitate developers to quickly implement customized backend management functions.

### 3.Features

* Gateway: Gateway server that supports tcp, kcp, ws and other protocols.
* Log: supports std, zap, logrus, aliyun, tencent and other log components.
* Registration: Supports multiple service registration centers such as consul, etcd, k8s, nacos, servicecomb, zookeeper, etc.
* Protocol: Supports json, protobuf (gogo/protobuf), msgpack and other communication protocols.
* Configuration: Supports json, yaml, toml, xml and other file formats.
* Communication: Supports various high-performance communication solutions such as grpc and rpcx.
* Restart: Supports smooth restart of the server.
* Event: Supports event bus implementation solutions such as redis, nats, kafka, rabbitMQ, etc.
* Encryption: Supports multiple encryption schemes such as rsa and ecc.
* Service: Supports various microservice solutions such as grpc and rpcx.
* Flexible: supports multiple architecture solutions such as single and distributed.
* Management: Provide master backend management service related interface support.

> Note: For performance reasons, the protobuf protocol uses [gogo/protobuf](https://github.com/gogo/protobuf) for encoding and decoding by default. When generating go code, please use protoc-gen-xxxx of the gogo library.

```bash
go install github.com/gogo/protobuf/protoc-gen-gofast@latest
```

### 4.Protocol

In the due framework, the communication protocol uniformly adopts the format of opcode+route+seq+message:

1.Data package format for websocket:

```
0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+-+-------------+-------------------------------+-------------------------------+
|h|   extcode   |             route             |              seq              |
+-+-------------+-------------------------------+-------------------------------+
|                                message data ...                               |
+-------------------------------------------------------------------------------+
```

2.Heartbeat package format for websocket:

```
0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+-+-------------+---------------------------------------------------------------+
|h|   extcode   |                       server time (ms)                        |
+-+-------------+---------------------------------------------------------------+
```

3.Data package format for tcp:

```
0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
|                              len                              |h|   extcode   |             route             |              seq              |
+---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
|                                                                message data ...                                                               |
+-----------------------------------------------------------------------------------------------------------------------------------------------+
```

4.Heartbeat package format for tcp:

```
0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
+---------------------------------------------------------------+-+-------------+---------------------------------------------------------------+
|                              len                              |h|   extcode   |                       server time (ms)                        |
+---------------------------------------------------------------+-+-------------+---------------------------------------------------------------+
```

5.Format description:

len: 4 bytes

- TCP package length bits
- Fixed length is 4 bytes and cannot be modified
- Use big endian order
- WebSocket protocol has no packet length bit
- This parameter is automatically packaged and generated by the TCP network framework. Server developers do not pay attention to this parameter. Client developers need to pay attention to this parameter.

h: 1 bit

- Heartbeat identification bit
- %x0 represents the data packet
- %x1 represents heartbeat packet
- Use big endian order
- This parameter is automatically packaged and generated by the network framework layer. Server developers do not pay attention to this parameter. Client developers need to pay attention to this parameter.

extcode: 7 bit

- Extended opcodes
- The specific operation code has not been clearly defined yet
- Use big endian order
- This parameter is automatically packaged and generated by the network framework layer. Server developers do not pay attention to this parameter. Client developers need to pay attention to this parameter.

route: 1 bytes | 2 bytes | 4 bytes

- Message routing
- The default is 2 bytes, which can be modified through the packager configuration packet.routeBytes
- Different routes correspond to different business processing processes
- Heartbeat packet has no message routing bit
- This parameter is packaged by the business packager. Both server developers and client developers should care about this parameter.

seq: 0 bytes | 1 bytes | 2 bytes | 4 bytes

- Message sequence number
- The default is 2 bytes, which can be modified through the packager configuration packet.seqBytes
- The use of sequence numbers can be blocked by setting the packer configuration packet.seqBytes to 0
- The message sequence number is often used to confirm the message pair of the request/response model.
- Heartbeat packet has no message sequence number bit
- This parameter is packaged by the business packager packet.Packer. Both server developers and client developers should care about this parameter.

message data: n bytes

- Message data
- Heartbeat packet has no message data
- This parameter is packaged by the business packager packet.Packer. Both server developers and client developers should care about this parameter.

server time: 8 bytes

- Heartbeat data
- Data packet has no heartbeat data
- Uplink heartbeat packets do not need to carry heartbeat data. Downlink heartbeat packets carry 8 bytes of server time (ms) by default. You can set whether to carry downlink packet time information through network library configuration.
- This parameter is automatically packaged by the network framework layer. Server developers do not pay attention to this parameter. Client developers need to pay attention to this parameter.

### 5.Configuration center

1.Feature introduction

The configuration center is mainly positioned at business configuration management and provides fast and flexible configuration solutions. Supports complete reading, modification, deletion, hot update and other functions.

2.Related components

* [file](config/file/README.md)
* [etcd](config/etcd/README.md)
* [consul](config/consul/README.md)

### 6.Registration center

1.Feature introduction

The registration center is used for service registration and discovery of cluster instances. Supports functions such as unconscious shutdown, restart, and dynamic expansion of the entire cluster.

2.Related components

* [etcd](registry/etcd/README.md)
* [consul](registry/consul/README.md)