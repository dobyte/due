package node_test

import (
	"context"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/node"
)

// deliverRecord is a message received by the test provider.
type deliverRecord struct {
	gid string
	nid string
	cid int64
	uid int64
	msg string
}

// provider records the messages delivered to it.
type provider struct {
	deliver chan deliverRecord
}

// Trigger triggers an event.
func (p *provider) Trigger(context.Context, string, int64, int64, cluster.Event) error {
	return nil
}

// Deliver delivers a message.
func (p *provider) Deliver(_ context.Context, gid, nid string, cid, uid int64, buf buffer.Buffer) error {
	p.deliver <- deliverRecord{gid: gid, nid: nid, cid: cid, uid: uid, msg: string(buf.Bytes())}

	return nil
}

// GetState returns the state.
func (p *provider) GetState() (cluster.State, error) {
	return cluster.Work, nil
}

// SetState sets the state.
func (p *provider) SetState(cluster.State) error {
	return nil
}

// newTestServer starts an in-process transporter server on a random local port, so that the tests
// neither bind a fixed port nor depend on an external peer.
func newTestServer(t *testing.T) (*node.Server, *provider) {
	t.Helper()

	p := &provider{deliver: make(chan deliverRecord, 8)}

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

	return server, p
}

func TestServer(t *testing.T) {
	server, _ := newTestServer(t)

	if server.ListenAddr() == "" {
		t.Fatal("expect a non-empty listen address")
	}
	if server.Scheme() != "drpc" {
		t.Fatalf("invalid scheme, expect: drpc, actual: %s", server.Scheme())
	}
}
