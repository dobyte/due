package gate_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// startGateServer starts an in-process gate server backed by p on a random local port.
func startGateServer(t *testing.T, p gate.Provider) *gate.Server {
	t.Helper()

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

	return server
}

// buildGateClient builds a gate client against addr.
func buildGateClient(t *testing.T, addr string, opts *gate.ClientOptions) *gate.Client {
	t.Helper()

	if opts == nil {
		opts = newTestClientOptions(xuuid.UUID())
	}

	client, err := gate.NewBuilder(opts).Build(addr)
	if err != nil {
		t.Fatalf("build client failed, err: %v", err)
	}

	return client
}

// unusedGateAddr returns a local address that is not being listened on.
func unusedGateAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed, err: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	return addr
}

// msg returns a fresh non-empty message buffer for a single client call.
func msg() buffer.Buffer {
	return buffer.NewNocopyBuffer([]byte("payload"))
}

// TestClientTransportAll exercises every gate client method against the in-process server.
func TestClientTransportAll(t *testing.T) {
	server, _ := newTestServer(t)
	client := buildGateClient(t, server.ListenAddr(), nil)
	ctx := context.Background()

	if err := client.Bind(ctx, 1, 2); err != nil {
		t.Errorf("Bind() error = %v", err)
	}
	if err := client.Unbind(ctx, 2); err != nil {
		t.Errorf("Unbind() error = %v", err)
	}
	if ip, err := client.GetIP(ctx, session.User, 1); err != nil || ip != providerIP {
		t.Errorf("GetIP() = (%q, %v), want (%q, nil)", ip, err, providerIP)
	}
	if total, err := client.Stat(ctx, session.Conn); err != nil || total != 0 {
		t.Errorf("Stat() = (%d, %v), want (0, nil)", total, err)
	}
	if isOnline, err := client.IsOnline(ctx, session.User, 1); err != nil || isOnline {
		t.Errorf("IsOnline() = (%v, %v), want (false, nil)", isOnline, err)
	}
	if err := client.Disconnect(ctx, session.User, 1, true); err != nil {
		t.Errorf("Disconnect() error = %v", err)
	}
	if err := client.Push(ctx, session.User, 1, false, msg(), true); err != nil {
		t.Errorf("Push(ack) error = %v", err)
	}
	if err := client.Push(ctx, session.User, 1, false, msg(), false); err != nil {
		t.Errorf("Push(no ack) error = %v", err)
	}
	if total, err := client.Multicast(ctx, session.User, []int64{1, 2}, false, msg(), true); err != nil || total != 0 {
		t.Errorf("Multicast(ack) = (%d, %v), want (0, nil)", total, err)
	}
	if total, err := client.Multicast(ctx, session.User, []int64{1, 2}, false, msg(), false); err != nil || total != 0 {
		t.Errorf("Multicast(no ack) = (%d, %v), want (0, nil)", total, err)
	}
	if total, err := client.Broadcast(ctx, session.User, false, msg(), true); err != nil || total != 0 {
		t.Errorf("Broadcast(ack) = (%d, %v), want (0, nil)", total, err)
	}
	if total, err := client.Broadcast(ctx, session.User, false, msg(), false); err != nil || total != 0 {
		t.Errorf("Broadcast(no ack) = (%d, %v), want (0, nil)", total, err)
	}
	if total, err := client.Publish(ctx, "channel", false, msg(), true); err != nil || total != 0 {
		t.Errorf("Publish(ack) = (%d, %v), want (0, nil)", total, err)
	}
	if total, err := client.Publish(ctx, "channel", false, msg(), false); err != nil || total != 0 {
		t.Errorf("Publish(no ack) = (%d, %v), want (0, nil)", total, err)
	}
	if err := client.Subscribe(ctx, session.User, []int64{1}, "channel"); err != nil {
		t.Errorf("Subscribe() error = %v", err)
	}
	if err := client.Unsubscribe(ctx, session.User, []int64{1}, "channel"); err != nil {
		t.Errorf("Unsubscribe() error = %v", err)
	}
	if err := client.SetState(ctx, cluster.Busy); err != nil {
		t.Errorf("SetState() error = %v", err)
	}
}

// TestClientInvalidArgument verifies the local argument validation of the client.
func TestClientInvalidArgument(t *testing.T) {
	server, _ := newTestServer(t)
	client := buildGateClient(t, server.ListenAddr(), nil)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "multicast too many targets",
			call: func() error {
				_, err := client.Multicast(ctx, session.User, make([]int64, 1<<16), false, msg(), true)
				return err
			},
		},
		{
			name: "publish channel too long",
			call: func() error {
				_, err := client.Publish(ctx, strings.Repeat("a", 1<<8), false, msg(), true)
				return err
			},
		},
		{
			name: "subscribe too many targets",
			call: func() error {
				return client.Subscribe(ctx, session.User, make([]int64, 1<<16), "channel")
			},
		},
		{
			name: "subscribe channel too long",
			call: func() error {
				return client.Subscribe(ctx, session.User, []int64{1}, strings.Repeat("a", 1<<8))
			},
		},
		{
			name: "unsubscribe too many targets",
			call: func() error {
				return client.Unsubscribe(ctx, session.User, make([]int64, 1<<16), "channel")
			},
		},
		{
			name: "unsubscribe channel too long",
			call: func() error {
				return client.Unsubscribe(ctx, session.User, []int64{1}, strings.Repeat("a", 1<<8))
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); !errors.Is(err, errors.ErrInvalidArgument) {
				t.Errorf("error = %v, want %v", err, errors.ErrInvalidArgument)
			}
		})
	}
}

// failingProvider is a provider whose methods always fail, so that the error code paths of the
// server handlers and the client are exercised.
type failingProvider struct {
	*provider
}

// Bind always fails.
func (p *failingProvider) Bind(context.Context, int64, int64) error { return errors.ErrNotFoundSession }

// Unbind always fails.
func (p *failingProvider) Unbind(context.Context, int64) error { return errors.ErrNotFoundSession }

// GetIP always fails.
func (p *failingProvider) GetIP(context.Context, session.Kind, int64) (string, error) {
	return "", errors.ErrNotFoundSession
}

// IsOnline always fails.
func (p *failingProvider) IsOnline(context.Context, session.Kind, int64) (bool, error) {
	return false, errors.ErrNotFoundSession
}

// Stat always fails.
func (p *failingProvider) Stat(context.Context, session.Kind) (int64, error) {
	return 0, errors.ErrNotFoundSession
}

// Disconnect always fails.
func (p *failingProvider) Disconnect(context.Context, session.Kind, int64, bool) error {
	return errors.ErrNotFoundSession
}

// Push always fails.
func (p *failingProvider) Push(context.Context, session.Kind, int64, bool, buffer.Buffer) error {
	return errors.ErrNotFoundSession
}

// Multicast always fails.
func (p *failingProvider) Multicast(context.Context, session.Kind, []int64, bool, buffer.Buffer) (int64, error) {
	return 0, errors.ErrNotFoundSession
}

// Broadcast always fails.
func (p *failingProvider) Broadcast(context.Context, session.Kind, bool, buffer.Buffer) (int64, error) {
	return 0, errors.ErrNotFoundSession
}

// Publish always fails.
func (p *failingProvider) Publish(context.Context, string, bool, buffer.Buffer) (int64, error) {
	return 0, errors.ErrNotFoundSession
}

// Subscribe always fails.
func (p *failingProvider) Subscribe(context.Context, session.Kind, []int64, string) error {
	return errors.ErrNotFoundSession
}

// Unsubscribe always fails.
func (p *failingProvider) Unsubscribe(context.Context, session.Kind, []int64, string) error {
	return errors.ErrNotFoundSession
}

// GetState always fails.
func (p *failingProvider) GetState() (cluster.State, error) { return 0, errors.ErrNotFoundSession }

// SetState always fails.
func (p *failingProvider) SetState(cluster.State) error { return errors.ErrNotFoundSession }

// TestClientProviderErrors verifies that a failing provider is reported to the client as an error
// code.
func TestClientProviderErrors(t *testing.T) {
	server := startGateServer(t, &failingProvider{provider: &provider{}})
	client := buildGateClient(t, server.ListenAddr(), nil)
	ctx := context.Background()

	calls := []struct {
		name string
		call func() error
	}{
		{"Bind", func() error { return client.Bind(ctx, 1, 2) }},
		{"Unbind", func() error { return client.Unbind(ctx, 2) }},
		{"GetIP", func() error { _, err := client.GetIP(ctx, session.User, 1); return err }},
		{"Stat", func() error { _, err := client.Stat(ctx, session.Conn); return err }},
		{"IsOnline", func() error { _, err := client.IsOnline(ctx, session.User, 1); return err }},
		{"Disconnect", func() error { return client.Disconnect(ctx, session.User, 1, false) }},
		{"Push", func() error { return client.Push(ctx, session.User, 1, false, msg(), true) }},
		{"Multicast", func() error {
			_, err := client.Multicast(ctx, session.User, []int64{1}, false, msg(), true)
			return err
		}},
		{"Broadcast", func() error { _, err := client.Broadcast(ctx, session.User, false, msg(), true); return err }},
		{"Publish", func() error { _, err := client.Publish(ctx, "channel", false, msg(), true); return err }},
		{"Subscribe", func() error { return client.Subscribe(ctx, session.User, []int64{1}, "channel") }},
		{"Unsubscribe", func() error { return client.Unsubscribe(ctx, session.User, []int64{1}, "channel") }},
		{"SetState", func() error { return client.SetState(ctx, cluster.Work) }},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); !errors.Is(err, errors.ErrNotFoundSession) {
				t.Errorf("error = %v, want %v", err, errors.ErrNotFoundSession)
			}
		})
	}
}

// TestClientNoConnection verifies that every client method reports a closed client when no
// connection has been established.
func TestClientNoConnection(t *testing.T) {
	cli, err := drpc.NewClient("127.0.0.1:1", &drpc.ClientOptions{
		ConnNum:        1,
		WriteQueueSize: 8,
		WriteTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("create drpc client failed, err: %v", err)
	}

	client := gate.NewClient(cli)
	ctx := context.Background()

	calls := []struct {
		name string
		call func() error
	}{
		{"Bind", func() error { return client.Bind(ctx, 1, 2) }},
		{"Unbind", func() error { return client.Unbind(ctx, 2) }},
		{"GetIP", func() error { _, err := client.GetIP(ctx, session.User, 1); return err }},
		{"Stat", func() error { _, err := client.Stat(ctx, session.Conn); return err }},
		{"IsOnline", func() error { _, err := client.IsOnline(ctx, session.User, 1); return err }},
		{"Disconnect", func() error { return client.Disconnect(ctx, session.User, 1, false) }},
		{"Push with ack", func() error { return client.Push(ctx, session.User, 1, false, msg(), true) }},
		{"Push without ack", func() error { return client.Push(ctx, session.User, 1, false, msg(), false) }},
		{"Multicast with ack", func() error {
			_, err := client.Multicast(ctx, session.User, []int64{1}, false, msg(), true)
			return err
		}},
		{"Multicast without ack", func() error {
			_, err := client.Multicast(ctx, session.User, []int64{1}, false, msg(), false)
			return err
		}},
		{"Broadcast with ack", func() error { _, err := client.Broadcast(ctx, session.User, false, msg(), true); return err }},
		{"Broadcast without ack", func() error {
			_, err := client.Broadcast(ctx, session.User, false, msg(), false)
			return err
		}},
		{"Publish with ack", func() error { _, err := client.Publish(ctx, "channel", false, msg(), true); return err }},
		{"Publish without ack", func() error { _, err := client.Publish(ctx, "channel", false, msg(), false); return err }},
		{"Subscribe", func() error { return client.Subscribe(ctx, session.User, []int64{1}, "channel") }},
		{"Unsubscribe", func() error { return client.Unsubscribe(ctx, session.User, []int64{1}, "channel") }},
		{"GetState", func() error { _, err := client.GetState(ctx); return err }},
		{"SetState", func() error { return client.SetState(ctx, cluster.Work) }},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); !errors.Is(err, errors.ErrClientClosed) {
				t.Errorf("error = %v, want %v", err, errors.ErrClientClosed)
			}
		})
	}
}

// TestServerAndBuilderErrors verifies the constructor error paths.
func TestServerAndBuilderErrors(t *testing.T) {
	if _, err := gate.NewServer(&provider{}, &gate.ServerOptions{Addr: "bad::addr"}); err == nil {
		t.Error("expect an error for an invalid server address")
	}

	builder := gate.NewBuilder(newTestClientOptions(xuuid.UUID()))
	if _, err := builder.Build("bad::addr"); err == nil {
		t.Error("expect an error for an invalid build address")
	}
}
