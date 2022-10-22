package node_test

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/transport/rpcx/node"
	"testing"
)

func TestNewClient(t *testing.T) {
	c, err := node.NewClient(endpoint.NewEndpoint("rpcx", "127.0.0.1:3554", false))
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Trigger(context.Background(), cluster.Disconnect, "1", 1)
	if err != nil {
		t.Fatal(err)
	}

	select {}
}
