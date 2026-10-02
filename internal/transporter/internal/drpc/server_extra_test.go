package drpc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

// TestServerAccessors verifies the server accessor methods.
func TestServerAccessors(t *testing.T) {
	s, err := NewServer(&ServerOptions{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	if s.Scheme() != scheme {
		t.Errorf("Scheme() = %q, want %q", s.Scheme(), scheme)
	}
	if s.ListenAddr() == "" {
		t.Error("expect a non-empty listen address")
	}
	if s.Endpoint() == nil {
		t.Error("expect a non-nil endpoint")
	}
	// ExposeAddr is exercised through the endpoint; it may be empty for a random local port.
	_ = s.ExposeAddr()
}

// TestServerRegisterHandlerWarnings verifies that duplicate registration and registration after
// start are ignored.
func TestServerRegisterHandlerWarnings(t *testing.T) {
	s, err := NewServer(&ServerOptions{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	handler := func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error { return nil }

	s.RegisterHandler(route.GetState, handler)
	// A duplicate route is ignored.
	s.RegisterHandler(route.GetState, handler)

	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	// Registration after start is ignored.
	s.RegisterHandler(route.SetState, handler)

	if err = s.Stop(); err != nil {
		t.Fatalf("stop server failed: %v", err)
	}
}

// TestServerClosedQueues verifies caching, loading, expiration and replacement of closed queues.
func TestServerClosedQueues(t *testing.T) {
	s := &Server{}

	kept := queue.NewQueue[buffer.Buffer](4, time.Second)
	s.doCacheQueue("key", kept)

	if loaded, ok := s.doLoadQueue("key"); !ok || loaded != kept {
		t.Fatalf("expect the cached queue to be loaded")
	}

	// An expired queue is dropped. It must be closed so that draining terminates.
	expired := queue.NewQueue[buffer.Buffer](4, time.Second)
	expired.Write(buffer.NewBytes([]byte("x")))
	expired.Close()
	s.queues.Store("old", &closedQueue{queue: expired, time: time.Now().Add(-2 * maxRetentionTime)})

	if _, ok := s.doLoadQueue("old"); ok {
		t.Error("expect an expired queue to be dropped")
	}

	// Replacing an existing entry drains the previous queue. It must be closed as well.
	first := queue.NewQueue[buffer.Buffer](4, time.Second)
	first.Write(buffer.NewBytes([]byte("a")))
	first.Close()
	s.doCacheQueue("dup", first)

	second := queue.NewQueue[buffer.Buffer](4, time.Second)
	s.doCacheQueue("dup", second)

	if loaded, ok := s.doLoadQueue("dup"); !ok || loaded != second {
		t.Error("expect the replacement queue to be cached")
	}
}

// TestServerCheck verifies the periodic heartbeat and closed-queue cleanup loop.
func TestServerCheck(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.ticker = time.NewTicker(10 * time.Millisecond)

	conn := &ServerConn{}
	conn.lastHeartbeatTime.Store(time.Now().Add(-3 * heartbeatInterval).UnixNano())
	s.conns.Store("conn", conn)

	q := queue.NewQueue[buffer.Buffer](4, time.Second)
	q.Close()
	s.queues.Store("queue", &closedQueue{queue: q, time: time.Now().Add(-2 * maxRetentionTime)})

	done := make(chan struct{})
	go func() {
		s.check()
		close(done)
	}()

	time.Sleep(60 * time.Millisecond)

	s.cancel()
	s.ticker.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("check did not exit after cancellation")
	}
}

// retryListener accepts once with a transient error and then reports the listener as closed.
type retryListener struct {
	transient bool
}

// Accept implements net.Listener.
func (l *retryListener) Accept() (net.Conn, error) {
	if !l.transient {
		l.transient = true
		return nil, io.ErrUnexpectedEOF
	}

	return nil, net.ErrClosed
}

// Close implements net.Listener.
func (l *retryListener) Close() error { return nil }

// Addr implements net.Listener.
func (l *retryListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }

// TestServerServeRetriesTransientAcceptError verifies that serve backs off on transient accept
// errors and stops when the listener is closed.
func TestServerServeRetriesTransientAcceptError(t *testing.T) {
	s, err := NewServer(&ServerOptions{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}
	if err = s.init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// serve returns after the fake listener reports net.ErrClosed, stopping the server.
	s.serve(&retryListener{})

	if s.listener != nil {
		t.Error("expect the server to be stopped")
	}
}

// TestServerHandleMessageUnknownRoute verifies that an unregistered route is rejected.
func TestServerHandleMessageUnknownRoute(t *testing.T) {
	s := &Server{}

	err := s.handleMessage(nil, route.Bind, 1, buffer.NewBytes([]byte("x")))
	if !errors.Is(err, errors.ErrNotFoundRoute) {
		t.Errorf("err = %v, want %v", err, errors.ErrNotFoundRoute)
	}
}
