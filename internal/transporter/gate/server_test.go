package gate_test

import (
	"context"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/session"
)

// providerIP is the address returned by the test provider.
const providerIP = "192.168.0.88"

type provider struct {
	gate.Provider
}

// Bind binds the relationship between the user and the gate.
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	return nil
}

// Unbind unbinds the relationship between the user and the gate.
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	return nil
}

// GetIP returns the client IP address.
func (p *provider) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, err error) {
	return providerIP, nil
}

// IsOnline reports whether the target is online.
func (p *provider) IsOnline(ctx context.Context, kind session.Kind, target int64) (isOnline bool, err error) {
	return
}

// Push sends a message (asynchronously).
func (p *provider) Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer) error {
	return nil
}

// Multicast pushes a multicast message (asynchronously).
func (p *provider) Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer) (total int64, err error) {
	return
}

// Broadcast pushes a broadcast message (asynchronously).
func (p *provider) Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer) (total int64, err error) {
	return
}

// Publish publishes a channel message (asynchronously).
func (p *provider) Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer) (total int64, err error) {
	return
}

// Subscribe subscribes to channels.
func (p *provider) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	return nil
}

// Unsubscribe unsubscribes from channels.
func (p *provider) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	return nil
}

// Stat returns the total number of sessions.
func (p *provider) Stat(ctx context.Context, kind session.Kind) (total int64, err error) {
	return
}

// Disconnect disconnects the target session.
func (p *provider) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	return nil
}

// GetState returns the state.
func (p *provider) GetState() (cluster.State, error) {
	return cluster.Work, nil
}

// SetState sets the state.
func (p *provider) SetState(state cluster.State) error {
	return nil
}

// newTestServer starts an in-process transporter server on a random local port, so that the tests
// neither bind a fixed port nor depend on an external peer.
func newTestServer(t *testing.T) (*gate.Server, *provider) {
	t.Helper()

	p := &provider{}

	server, err := gate.NewServer(p, &gate.ServerOptions{Addr: "127.0.0.1:0"})
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
