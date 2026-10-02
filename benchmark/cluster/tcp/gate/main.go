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
	// Create the container
	container := due.NewContainer()
	// Create the server
	server := tcp.NewServer()
	// Create the user locator
	locator := redis.NewLocator()
	// Create the service registry
	registry := nacos.NewRegistry()
	// Create the gate component
	component1 := gate.NewGate(
		gate.WithServer(server),
		gate.WithLocator(locator),
		gate.WithRegistry(registry),
	)
	// Create the pprof component
	component2 := pprof.NewPProf()
	// Add the gate component
	container.Add(component1, component2)
	// Start the container
	container.Serve()
}
