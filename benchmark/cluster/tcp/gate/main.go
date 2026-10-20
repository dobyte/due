package main

import (
	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/component/pprof"
	"github.com/dobyte/due/v2/flag"
)

func main() {
	// Enable security
	security := flag.Bool("security", false)
	// Create the gate options
	opts := make([]gate.Option, 0, 4)
	// Create the server
	server := tcp.NewServer()
	// Create the gate server
	opts = append(opts, gate.WithServer(server))
	// Create the user locator
	opts = append(opts, gate.WithLocator(redis.NewLocator()))
	// Create the service registry
	opts = append(opts, gate.WithRegistry(nacos.NewRegistry()))
	// Create the session security
	if security {
		opts = append(opts, gate.WithSessionSecurity(ecc.NewSigner()))
	}
	// Create the gate component
	component1 := gate.NewGate(opts...)
	// Create the pprof component
	component2 := pprof.NewPProf()
	// Create the container
	container := due.NewContainer()
	// Add the gate component
	container.Add(component1, component2)
	// Start the container
	container.Serve()
}
