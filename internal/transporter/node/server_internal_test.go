package node

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/session"
)

// internalProvider is a minimal provider used to exercise the server handlers directly.
type internalProvider struct {
	state cluster.State
}

// Trigger triggers an event.
func (p *internalProvider) Trigger(context.Context, string, int64, int64, cluster.Event) error {
	return nil
}

// Deliver delivers a message.
func (p *internalProvider) Deliver(context.Context, string, string, int64, int64, buffer.Buffer) error {
	return nil
}

// GetState returns the state.
func (p *internalProvider) GetState() (cluster.State, error) { return p.state, nil }

// SetState sets the state.
func (p *internalProvider) SetState(cluster.State) error { return nil }

// captureServerConn starts a server that records the first connection reaching the Push route. The
// captured connection lets the internal tests drive the request/response handlers that the client
// cannot reach over the wire (its requests carry an empty private section and are treated as
// keep-alive packets).
func captureServerConn(t *testing.T, p Provider) (*Server, *drpc.ServerConn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	s, err := NewServer(p, &ServerOptions{Addr: addr, WriteQueueSize: 1024, WriteTimeout: time.Second})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	conns := make(chan *drpc.ServerConn, 1)
	s.RegisterHandler(route.Push, func(conn *drpc.ServerConn, seq uint64, buf *buffer.Bytes) error {
		buf.Release()

		select {
		case conns <- conn:
		default:
		}

		return nil
	})

	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })

	cli, err := drpc.NewClient(addr, &drpc.ClientOptions{
		ID:             "internal",
		Kind:           cluster.Gate,
		ConnNum:        1,
		DialTimeout:    time.Second,
		DialRetryTimes: 3,
		WriteTimeout:   time.Second,
		WriteQueueSize: 1024,
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if err = cli.Establish(); err != nil {
		t.Fatalf("establish failed: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	if err = cli.Push(context.Background(), protocol.EncodePushReq(0, session.User, 1, false, buffer.NewBytes(nil)), 1); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	select {
	case conn := <-conns:
		return s, conn
	case <-time.After(3 * time.Second):
		t.Fatal("the server connection is not captured")
		return nil, nil
	}
}

// TestServerGetStateNoResponse verifies the branches of getState that write no response.
func TestServerGetStateNoResponse(t *testing.T) {
	s := &Server{provider: &internalProvider{state: cluster.Busy}}

	if err := s.getState(nil, 0, buffer.NewBytes(nil)); err != nil {
		t.Errorf("getState with a valid request error = %v", err)
	}
	if err := s.getState(nil, 0, buffer.NewBytes([]byte{1})); err == nil {
		t.Error("expect an error for a non-empty get-state request")
	}
}

// TestServerGetStateResponse verifies that getState pushes a response for a non-zero sequence.
func TestServerGetStateResponse(t *testing.T) {
	s, conn := captureServerConn(t, &internalProvider{state: cluster.Busy})

	if err := s.getState(conn, 1, buffer.NewBytes(nil)); err != nil {
		t.Errorf("getState with a non-zero sequence error = %v", err)
	}
}

// TestServerSetStateHandler verifies the setState handler.
func TestServerSetStateHandler(t *testing.T) {
	s, conn := captureServerConn(t, &internalProvider{})

	if err := s.setState(conn, 1, buffer.NewBytes([]byte{byte(cluster.Busy)})); err != nil {
		t.Errorf("setState error = %v", err)
	}
	if err := s.setState(nil, 0, buffer.NewBytes([]byte{byte(cluster.Busy)})); err != nil {
		t.Errorf("setState without ack error = %v", err)
	}
	if err := s.setState(nil, 0, buffer.NewBytes([]byte{1, 2})); err == nil {
		t.Error("expect an error for a malformed set-state request")
	}
}

// TestServerTriggerDecodeError verifies that a malformed trigger request is rejected before the
// connection is inspected.
func TestServerTriggerDecodeError(t *testing.T) {
	s := &Server{provider: &internalProvider{}}

	if err := s.trigger(nil, 0, buffer.NewBytes([]byte{1})); err == nil {
		t.Error("expect an error for a malformed trigger request")
	}
}
