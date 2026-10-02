package node_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// triggerRecord is a trigger received by the test provider.
type triggerRecord struct {
	gid   string
	cid   int64
	uid   int64
	event cluster.Event
}

// recordingProvider records the triggers delivered to it and reuses the message provider.
type recordingProvider struct {
	*provider
	triggers chan triggerRecord
}

// Trigger triggers an event.
func (p *recordingProvider) Trigger(_ context.Context, gid string, cid, uid int64, event cluster.Event) error {
	p.triggers <- triggerRecord{gid: gid, cid: cid, uid: uid, event: event}

	return nil
}

// newProvider returns a provider with buffered delivery and trigger channels.
func newProvider() *recordingProvider {
	return &recordingProvider{
		provider: &provider{deliver: make(chan deliverRecord, 8)},
		triggers: make(chan triggerRecord, 8),
	}
}

// newNodeServer starts an in-process node server backed by p on a random local port.
func newNodeServer(t *testing.T, p node.Provider) *node.Server {
	t.Helper()

	server, err := node.NewServer(p, &node.ServerOptions{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("create server failed, err: %v", err)
	}
	if err = server.Start(); err != nil {
		t.Fatalf("start server failed, err: %v", err)
	}

	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Logf("stop server failed, err: %v", err)
		}
	})

	return server
}

// clientOptionsForKind returns client options whose instance kind is kind.
func clientOptionsForKind(id string, kind cluster.Kind) *node.ClientOptions {
	opts := newTestClientOptions(id)
	opts.Kind = kind

	return opts
}

// buildNodeClient builds a node client against addr.
func buildNodeClient(t *testing.T, addr string, opts *node.ClientOptions) *node.Client {
	t.Helper()

	client, err := node.NewBuilder(opts).Build(addr)
	if err != nil {
		t.Fatalf("build client failed, err: %v", err)
	}

	return client
}

// unusedNodeAddr returns a local address that is not being listened on.
func unusedNodeAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed, err: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	return addr
}

// TestNodeClientTrigger verifies that a gate connection triggers events on the provider.
func TestNodeClientTrigger(t *testing.T) {
	p := newProvider()
	server := newNodeServer(t, p)

	id := xuuid.UUID()
	client := buildNodeClient(t, server.ListenAddr(), clientOptionsForKind(id, cluster.Gate))

	if err := client.Trigger(context.Background(), cluster.Connect, 1, 2); err != nil {
		t.Fatalf("trigger failed, err: %v", err)
	}

	select {
	case record := <-p.triggers:
		if record.gid != id {
			t.Errorf("invalid gate id, expect: %s, actual: %s", id, record.gid)
		}
		if record.cid != 1 || record.uid != 2 {
			t.Errorf("invalid session, cid: %d, uid: %d", record.cid, record.uid)
		}
		if record.event != cluster.Connect {
			t.Errorf("invalid event, expect: %v, actual: %v", cluster.Connect, record.event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the event is not triggered")
	}
}

// TestNodeClientDeliverFromNode verifies that a node connection delivers with the node id set.
func TestNodeClientDeliverFromNode(t *testing.T) {
	p := newProvider()
	server := newNodeServer(t, p)

	id := xuuid.UUID()
	client := buildNodeClient(t, server.ListenAddr(), clientOptionsForKind(id, cluster.Node))

	if err := client.Deliver(context.Background(), 1, 2, buffer.NewNocopyBuffer([]byte("hello"))); err != nil {
		t.Fatalf("deliver failed, err: %v", err)
	}

	select {
	case record := <-p.deliver:
		if record.nid != id {
			t.Errorf("invalid node id, expect: %s, actual: %s", id, record.nid)
		}
		if record.gid != "" {
			t.Errorf("invalid gate id, expect empty, actual: %s", record.gid)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the message is not delivered")
	}
}

// TestNodeClientState verifies the state round trip.
func TestNodeClientState(t *testing.T) {
	p := newProvider()
	server := newNodeServer(t, p)

	client := buildNodeClient(t, server.ListenAddr(), clientOptionsForKind(xuuid.UUID(), cluster.Gate))

	if err := client.SetState(context.Background(), cluster.Busy); err != nil {
		t.Errorf("set state failed, err: %v", err)
	}
}

// TestNodeClientDeliverRejected verifies that a connection of an unsupported kind cannot deliver.
func TestNodeClientDeliverRejected(t *testing.T) {
	p := newProvider()
	server := newNodeServer(t, p)

	client := buildNodeClient(t, server.ListenAddr(), clientOptionsForKind(xuuid.UUID(), cluster.Mesh))

	if err := client.Deliver(context.Background(), 1, 2, buffer.NewNocopyBuffer([]byte("hello"))); err != nil {
		t.Fatalf("deliver failed, err: %v", err)
	}

	select {
	case <-p.deliver:
		t.Fatal("a mesh connection must not deliver messages")
	case <-time.After(300 * time.Millisecond):
	}
}

// TestNodeClientTriggerRejected verifies that a connection that is not a gate cannot trigger
// events.
func TestNodeClientTriggerRejected(t *testing.T) {
	p := newProvider()
	server := newNodeServer(t, p)

	client := buildNodeClient(t, server.ListenAddr(), clientOptionsForKind(xuuid.UUID(), cluster.Node))

	if err := client.Trigger(context.Background(), cluster.Connect, 1, 2); err != nil {
		t.Fatalf("trigger failed, err: %v", err)
	}

	select {
	case <-p.triggers:
		t.Fatal("a node connection must not trigger events")
	case <-time.After(300 * time.Millisecond):
	}
}

// TestNodeBuilderErrors verifies the builder error paths.
func TestNodeBuilderErrors(t *testing.T) {
	if _, err := node.NewServer(&provider{deliver: make(chan deliverRecord, 1)}, &node.ServerOptions{Addr: "bad::addr"}); err == nil {
		t.Error("expect an error for an invalid server address")
	}

	if _, err := node.NewBuilder(newTestClientOptions(xuuid.UUID())).Build("bad::addr"); err == nil {
		t.Error("expect an error for an invalid build address")
	}

	opts := clientOptionsForKind(xuuid.UUID(), cluster.Gate)
	opts.DialTimeout = 100 * time.Millisecond
	opts.DialRetryTimes = 0

	if _, err := node.NewBuilder(opts).Build(unusedNodeAddr(t)); err == nil {
		t.Error("expect an error for an unreachable endpoint")
	}
}

// TestNodeClientNoConnection verifies that every client method reports a closed client when no
// connection has been established.
func TestNodeClientNoConnection(t *testing.T) {
	cli, err := drpc.NewClient("127.0.0.1:1", &drpc.ClientOptions{
		ConnNum:        1,
		WriteQueueSize: 8,
		WriteTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("create drpc client failed, err: %v", err)
	}

	client := node.NewClient(cli)
	ctx := context.Background()

	if err = client.Trigger(ctx, cluster.Connect, 1, 2); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("Trigger() error = %v, want %v", err, errors.ErrClientClosed)
	}
	if err = client.Deliver(ctx, 1, 2, buffer.NewNocopyBuffer([]byte("x"))); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("Deliver() error = %v, want %v", err, errors.ErrClientClosed)
	}
	if _, err = client.GetState(ctx); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("GetState() error = %v, want %v", err, errors.ErrClientClosed)
	}
	if err = client.SetState(ctx, cluster.Work); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("SetState() error = %v, want %v", err, errors.ErrClientClosed)
	}
}
