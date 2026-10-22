// Command bridge runs a single-process Gate and Node without external services.
package main

import (
	"flag"
	"fmt"

	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/encoding/json"
	locatememory "github.com/dobyte/due/v2/locate/memory"
	"github.com/dobyte/due/v2/log"
	registrymemory "github.com/dobyte/due/v2/registry/memory"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:3553", "player TCP listen address")
	remote := flag.Bool("drpc", false, "use loopback DRPC instead of the local bridge")
	flag.Parse()
	r := registrymemory.NewRegistry()
	l := locatememory.NewLocator()
	defer r.Close()
	defer l.Close()
	n := node.NewNode(node.WithID("node-1"), node.WithRegistry(r), node.WithLocator(l),
		node.WithCodec(json.DefaultCodec), node.WithLocalBridge(!*remote), node.WithLinkerListen(*remote))
	n.Proxy().AddRouteHandler(1, func(ctx node.Context) {
		var message struct {
			Text string `json:"text"`
		}
		if err := ctx.Parse(&message); err != nil {
			log.Warnf("parse echo message: %v", err)
			return
		}
		if err := ctx.Response(message); err != nil {
			log.Warnf("respond to echo message: %v", err)
		}
	})
	g := gate.NewGate(gate.WithID("gate-1"), gate.WithRegistry(r), gate.WithLocator(l),
		gate.WithServer(tcp.NewServer(tcp.WithServerAddr(*addr))),
		gate.WithLocalBridge(!*remote), gate.WithLinkerListen(*remote))
	fmt.Printf("Echo route 1, JSON payload {\"text\":\"hello\"}, TCP %s\n", *addr)
	c := due.NewContainer()
	c.Add(n, g)
	c.Serve()
}
