package main

import (
	"sync"

	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/component/pprof"
	"github.com/dobyte/due/v2/log"
)

const greet = 1

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := nacos.NewRegistry()
	// 创建节点组件
	component1 := node.NewNode(
		node.WithLocator(locator),
		node.WithRegistry(registry),
	)
	// 创建PProf组件
	component2 := pprof.NewPProf()
	// 初始化监听
	initListen(component1.Proxy())
	// 添加节点组件
	container.Add(component1, component2)
	// 启动容器
	container.Serve()
}

// 初始化监听
func initListen(proxy *node.Proxy) {
	proxy.Router().AddRouteHandler(greet, greetHandler)
}

type greetReq struct {
	Message string `json:"message"`
}

type greetRes struct {
	Message string `json:"message"`
}

var reqPool = sync.Pool{New: func() any {
	return &greetReq{}
}}

var resPool = sync.Pool{New: func() any {
	return &greetRes{}
}}

func greetHandler(ctx node.Context) {
	req := &greetReq{}
	res := &greetRes{}

	ctx.Defer(func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response message failed: %v", err)
		}
	})

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse request message failed: %v", err)
		return
	}

	res.Message = req.Message
}
