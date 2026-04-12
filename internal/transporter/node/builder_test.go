package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/utils/xuuid"
)

func TestBuilder(t *testing.T) {
	builder := node.NewBuilder(&node.ClientOptions{
		ID:                xuuid.UUID(),
		Kind:              cluster.Gate,
		ConnNum:           10,
		DialTimeout:       3 * time.Second,
		DialRetryTimes:    3,
		WriteTimeout:      1 * time.Second,
		WriteQueueSize:    1024,
		CallTimeout:       3 * time.Second,
		FaultRecoveryTime: 3 * time.Second,
	})

	client, err := builder.Build("127.0.0.1:49898")
	if err != nil {
		t.Fatal(err)
	}

	err = client.Deliver(context.Background(), 1, 2, buffer.NewNocopyBuffer([]byte("hello world")))
	if err != nil {
		t.Fatal(err)
	}
}
