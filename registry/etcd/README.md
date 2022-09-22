# due-registry-etcd

### 1.功能

* 支持完整的服务注册发现接口
* 支持多线程模式下服务实例的注册与监听
* 支持服务实例的更新操作
* 支持内建客户端和注入外部客户端两种连接方式
* 支持心跳重试机制

### 2.快速开始

1.安装

```shell
go get github.com/dobyte/due/registry/etcd@latest
```

2.开始使用

```go
package main

import (
	"context"
	"time"
	
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/registry/etcd"
)

func main() {
	reg := etcd.NewRegistry()
	
	watch(reg, cluster.Node.String(), 1)
	watch(reg, cluster.Node.String(), 2)
	
	ins := &registry.ServiceInstance{
		ID:       "test-1",
		Name:     "login-server",
		Kind:     cluster.Node,
		State:    cluster.Work,
		Endpoint: "grpc://127.0.0.1:6339",
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := reg.Register(ctx, ins)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	
	time.Sleep(2 * time.Second)
	
	ins.State = cluster.Busy
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	err = reg.Register(ctx, ins)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	
	time.Sleep(20 * time.Second)
	
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	err = reg.Deregister(ctx, ins)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	
	time.Sleep(40 * time.Second)
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