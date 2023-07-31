# 注册中心-etcd

### 1.功能

* 支持完整的服务注册发现接口
* 支持多线程模式下服务实例的注册与监听
* 支持服务实例的更新操作
* 支持内建客户端和注入外部客户端两种连接方式
* 支持心跳重试机制

### 2.快速开始

1.安装

```shell
go get github.com/dobyte/due/registry/etcd/v2@latest
```

2.etc配置项

```toml
# 注册中心
[registry]
    # etcd注册中心
    [registry.etcd]
        # 客户端连接地址
        addrs = ["127.0.0.1:2379"]
        # 客户端拨号超时时间（秒）
        dialTimeout = 5
        # 命名空间
        namespace = "services"
        # 超时时间（秒）
        timeout = 3
        # 心跳重试次数
        retryTimes = 3
        # 心跳重试间隔（秒）
        retryInterval = 10
```

3.开始使用

```go
package main

import (
	"context"
	"github.com/dobyte/due/registry/etcd/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xuuid"
	"time"
)

func main() {
	var (
		reg   = etcd.NewRegistry()
		id, _ = xuuid.UUID()
		name  = "game-server"
		alias = "mahjong"
		ins   = &registry.ServiceInstance{
			ID:       id,
			Name:     name,
			Kind:     cluster.Node,
			Alias:    alias,
			State:    cluster.Work,
			Endpoint: "grpc://127.0.0.1:6339",
		}
	)

	// 监听
	watch(reg, name, 1)
	watch(reg, name, 2)

	// 注册服务
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := reg.Register(ctx, ins)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	// 更新服务
	ins.State = cluster.Busy
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	err = reg.Register(ctx, ins)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	// 解注册服务
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	err = reg.Deregister(ctx, ins)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(10 * time.Second)
}

func watch(reg *etcd.Registry, serviceName string, goroutineID int) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	watcher, err := reg.Watch(ctx, serviceName)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			services, err := watcher.Next()
			if err != nil {
				log.Fatalf("goroutine %d: %v", goroutineID, err)
				return
			}

			log.Infof("goroutine %d: new event entity", goroutineID)

			for _, service := range services {
				log.Infof("goroutine %d: %+v", goroutineID, service)
			}
		}
	}()
}
```

### 3.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples/registry/etcd/README.md)