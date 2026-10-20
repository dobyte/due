package main

import (
	"due-benchmark/cluster/ws-security/internal/profile"
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/log"
)

func main() {
	profile.ConfigurePacket()
	component := node.NewNode(
		node.WithLocator(redis.NewLocator()),
		node.WithRegistry(nacos.NewRegistry()),
	)
	component.Proxy().Router().AddRouteHandler(profile.Greet, greetHandler)
	container := due.NewContainer()
	container.Add(component)
	container.Serve()
}

func greetHandler(ctx node.Context) {
	var req profile.Greeting
	if err := ctx.Parse(&req); err != nil {
		log.Errorf("parse request: %v", err)
		return
	}
	if err := ctx.Response(&req); err != nil {
		log.Errorf("echo response: %v", err)
	}
}
