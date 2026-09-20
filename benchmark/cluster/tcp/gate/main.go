package main

import (
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/component/pprof"
)

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建服务器
	server := tcp.NewServer()
	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := nacos.NewRegistry()
	// 创建网关组件
	component1 := gate.NewGate(
		gate.WithServer(server),
		gate.WithLocator(locator),
		gate.WithRegistry(registry),
	)
	// 创建PProf组件
	component2 := pprof.NewPProf()
	// 添加网关组件
	container.Add(component1, component2)
	// 启动容器
	container.Serve()
}
