package node_test

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/internal/endpoint"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/rpcx/node"
	"testing"
)

func TestNewClient(t *testing.T) {
	ep := endpoint.NewEndpoint("rpcx", "127.0.0.1:3554", false)

	c, err := node.NewClient(ep)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Trigger(context.Background(), &transport.TriggerArgs{
		GID:   "1",
		UID:   1,
		Event: cluster.Disconnect,
	})
	if err != nil {
		t.Fatal(err)
	}

	select {}
}
