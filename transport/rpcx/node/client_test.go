package node_test

import (
	"context"
	"github.com/dobyte/due/transport/rpcx/v2/internal/client"
	"github.com/dobyte/due/transport/rpcx/v2/node"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/transport"
	"testing"
)

func TestNewClient(t *testing.T) {
	ep := endpoint.NewEndpoint("rpcx", "127.0.0.1:3554", false)
	builder := client.NewBuilder(&client.Options{
		PoolSize: 10,
	})

	cli, err := builder.Build(ep.Target())
	if err != nil {
		t.Fatal(err)
	}

	c := node.NewClient(cli)

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
