package gate_test

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// newTestClientOptions returns client options that fail fast, keeping the tests short.
func newTestClientOptions(id string) *gate.ClientOptions {
	return &gate.ClientOptions{
		ID:                id,
		Kind:              cluster.Node,
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
	server, _ := newTestServer(t)

	builder := gate.NewBuilder(newTestClientOptions(xuuid.UUID()))

	client, err := builder.Build(server.ListenAddr())
	if err != nil {
		t.Fatalf("build client failed, err: %v", err)
	}

	ctx := context.Background()

	ip, err := client.GetIP(ctx, cluster.User, 1)
	if err != nil {
		t.Fatalf("get ip failed, err: %v", err)
	}
	if ip != providerIP {
		t.Fatalf("invalid ip, expect: %s, actual: %s", providerIP, ip)
	}

	// Building for the same address reuses the cached client.
	again, err := builder.Build(server.ListenAddr())
	if err != nil {
		t.Fatalf("build client failed, err: %v", err)
	}
	if again != client {
		t.Fatal("expect the cached client to be reused")
	}

	// Closing the builder closes the cached clients and rejects new builds.
	if err = builder.Close(); err != nil {
		t.Fatalf("close builder failed, err: %v", err)
	}

	if _, err = builder.Build(server.ListenAddr()); !errors.Is(err, errors.ErrClientClosed) {
		t.Fatalf("expect ErrClientClosed on build, got %v", err)
	}

	if err = builder.Close(); err != nil {
		t.Fatalf("closing the builder twice failed, err: %v", err)
	}
}
