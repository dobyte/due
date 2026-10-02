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
	// Create the container
	container := due.NewContainer()
	// Create the user locator
	locator := redis.NewLocator()
	// Create the service registry
	registry := nacos.NewRegistry()
	// Create the node component
	component1 := node.NewNode(
		node.WithLocator(locator),
		node.WithRegistry(registry),
	)
	// Create the pprof component
	component2 := pprof.NewPProf()
	// Initialize the listeners
	initListen(component1.Proxy())
	// Add the node component
	container.Add(component1, component2)
	// Start the container
	container.Serve()
}

// initListen initializes the listeners.
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
