package main

import (
	"net/http"

	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/component/pprof"
	"github.com/gorilla/websocket"
)

func main() {
	// Create the container
	container := due.NewContainer()
	// Create the server
	server := ws.NewServer()

	// Check whether the request is a valid WebSocket request before the upgrade; a non-WebSocket
	// request (such as a direct browser visit) is answered with 426 to avoid triggering upgrade
	// error logs
	server.OnUpgrade(func(w http.ResponseWriter, r *http.Request) (allowed bool) {
		if websocket.IsWebSocketUpgrade(r) {
			return true
		}
		http.Error(w, "websocket connection required", http.StatusUpgradeRequired)
		return false
	})

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
