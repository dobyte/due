package drpc

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

// newEchoServer starts a drpc server whose SetState handler replies with a success code and whose
// Deliver handler consumes messages without replying.
//
// SetState is used as the echo route because its request carries a non-empty private section;
// empty private sections are intentionally dropped by the read loop as keep-alive packets.
func newEchoServer(t *testing.T) *Server {
	t.Helper()

	addr := listenFreeAddr(t)
	s, err := NewServer(&ServerOptions{Addr: addr, WriteQueueSize: 1024, WriteTimeout: time.Second})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	s.RegisterHandler(route.SetState, func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error {
		buf.Release()
		if seq == 0 {
			return nil
		}

		return conn.Push(protocol.EncodeSetStateRes(seq, codes.OK))
	})
	s.RegisterHandler(route.Deliver, func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error {
		buf.Release()
		return nil
	})

	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })

	return s
}

// newTestDRPCClient creates and establishes a client against addr.
func newTestDRPCClient(t *testing.T, addr string, opts *ClientOptions) *Client {
	t.Helper()

	if opts == nil {
		opts = &ClientOptions{
			ID:             "test-client",
			Kind:           cluster.Gate,
			ConnNum:        1,
			DialTimeout:    time.Second,
			DialRetryTimes: 3,
			WriteTimeout:   time.Second,
			WriteQueueSize: 1024,
			CallTimeout:    2 * time.Second,
		}
	}

	cli, err := NewClient(addr, opts)
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if err = cli.Establish(); err != nil {
		t.Fatalf("establish failed: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	return cli
}

// TestClientCallRoundTrip verifies a full call round trip, covering both the explicit index and
// round-robin connection selection.
func TestClientCallRoundTrip(t *testing.T) {
	s := newEchoServer(t)
	cli := newTestDRPCClient(t, s.ListenAddr(), nil)
	ctx := context.Background()

	// Explicit connection index.
	res, err := cli.Call(ctx, 2, protocol.EncodeSetStateReq(2, cluster.Busy), 0)
	if err != nil {
		t.Fatalf("call with an explicit index failed: %v", err)
	}
	code, err := protocol.DecodeSetStateRes(res)
	res.Release()
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if code != codes.OK {
		t.Fatalf("unexpected response: code = %d", code)
	}

	// Round-robin selection.
	res, err = cli.Call(ctx, 3, protocol.EncodeSetStateReq(3, cluster.Work))
	if err != nil {
		t.Fatalf("call with round-robin selection failed: %v", err)
	}
	res.Release()
}

// TestClientPush verifies fire-and-forget pushes.
func TestClientPush(t *testing.T) {
	s := newEchoServer(t)
	cli := newTestDRPCClient(t, s.ListenAddr(), nil)

	if err := cli.Push(context.Background(), protocol.EncodeDeliverReq(0, 1, 2, buffer.NewBytes(nil)), 1); err != nil {
		t.Fatalf("push failed: %v", err)
	}
}

// TestClientWithoutConn verifies that Call and Push fail cleanly when no connection has been
// established and when the context is already canceled.
func TestClientWithoutConn(t *testing.T) {
	opts := &ClientOptions{ConnNum: 1, WriteQueueSize: 8, WriteTimeout: time.Second}

	cli, err := NewClient("127.0.0.1:1", opts)
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}

	if _, err = cli.Call(context.Background(), 1, protocol.EncodeGetStateReq(1)); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("call: err = %v, want %v", err, errors.ErrClientClosed)
	}
	if err = cli.Push(context.Background(), protocol.EncodeGetStateReq(1)); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("push: err = %v, want %v", err, errors.ErrClientClosed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = cli.Call(ctx, 1, protocol.EncodeGetStateReq(1)); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled call: err = %v, want %v", err, context.Canceled)
	}
}

// TestClientCallTimeout verifies that a call without a response times out and is discarded.
func TestClientCallTimeout(t *testing.T) {
	addr := listenFreeAddr(t)
	s, err := NewServer(&ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}
	// No handler is registered for Bind, so the server never replies.
	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })

	cli := newTestDRPCClient(t, addr, &ClientOptions{
		ID:             "test-client",
		Kind:           cluster.Gate,
		ConnNum:        1,
		DialTimeout:    time.Second,
		DialRetryTimes: 3,
		WriteTimeout:   time.Second,
		WriteQueueSize: 1024,
		CallTimeout:    100 * time.Millisecond,
	})

	start := time.Now()
	if _, err = cli.Call(context.Background(), 5, protocol.EncodeBindReq(5, 1, 2)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("call timeout not honored, elapsed: %v", elapsed)
	}
}
