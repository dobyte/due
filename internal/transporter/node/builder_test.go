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

// newTestClientOptions returns client options that fail fast, keeping the tests short.
func newTestClientOptions(id string) *node.ClientOptions {
	return &node.ClientOptions{
		ID:                id,
		Kind:              cluster.Gate,
		ConnNum:           10,
		DialTimeout:       3 * time.Second,
		DialRetryTimes:    3,
		WriteTimeout:      time.Second,
		WriteQueueSize:    1024,
		CallTimeout:       3 * time.Second,
		FaultRecoveryTime: 3 * time.Second,
	}
}

func TestBuilder(t *testing.T) {
	server, p := newTestServer(t)

	id := xuuid.UUID()
	builder := node.NewBuilder(newTestClientOptions(id))

	client, err := builder.Build(server.ListenAddr())
	if err != nil {
		t.Fatalf("build client failed, err: %v", err)
	}

	if err = client.Deliver(context.Background(), 1, 2, buffer.NewNocopyBuffer([]byte("hello world"))); err != nil {
		t.Fatalf("deliver message failed, err: %v", err)
	}

	select {
	case record := <-p.deliver:
		if record.gid != id {
			t.Fatalf("invalid gate id, expect: %s, actual: %s", id, record.gid)
		}
		if record.cid != 1 || record.uid != 2 {
			t.Fatalf("invalid session, cid: %d, uid: %d", record.cid, record.uid)
		}
		if record.msg != "hello world" {
			t.Fatalf("invalid message, expect: hello world, actual: %s", record.msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the message is not delivered")
	}

	// Building for the same address reuses the cached client.
	again, err := builder.Build(server.ListenAddr())
	if err != nil {
		t.Fatalf("build client failed, err: %v", err)
	}
	if again != client {
		t.Fatal("expect the cached client to be reused")
	}
}
