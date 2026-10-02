package drpc

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

// newIdleClientConn returns a client and a connection whose options are safe for offline use.
func newIdleClientConn() (*Client, *ClientConn) {
	cli := &Client{opts: &ClientOptions{
		WriteQueueSize: 128,
		WriteTimeout:   time.Second,
		CallTimeout:    time.Second,
	}}

	return cli, newClientConn(cli)
}

// TestClientGenEpochWrap verifies that the epoch generator skips the zero value.
func TestClientGenEpochWrap(t *testing.T) {
	cli := &Client{opts: &ClientOptions{WriteQueueSize: 8, WriteTimeout: time.Second}}
	cli.epoch.Store(math.MaxUint64)

	if epoch := cli.doGenEpoch(); epoch != 1 {
		t.Fatalf("expect epoch 1 after wrap, got %d", epoch)
	}
}

// TestClientLoadConn verifies connection selection, including the index and round-robin paths.
func TestClientLoadConn(t *testing.T) {
	cli := &Client{}

	if _, err := cli.doLoadConn(context.Background()); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("empty client: err = %v, want %v", err, errors.ErrClientClosed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cli.doLoadConn(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context: err = %v, want %v", err, context.Canceled)
	}

	pool := &Client{conns: []*ClientConn{{}, {}, {}}}

	if conn, err := pool.doLoadConn(context.Background(), 1); err != nil || conn != pool.conns[1] {
		t.Errorf("explicit index: conn = %p, err = %v", conn, err)
	}
	// A negative index falls back to round-robin selection.
	if conn, err := pool.doLoadConn(context.Background(), -1); err != nil || conn == nil {
		t.Errorf("negative index: conn = %p, err = %v", conn, err)
	}

	first, _ := pool.doLoadConn(context.Background())
	second, _ := pool.doLoadConn(context.Background())
	if first == second {
		t.Error("expect round-robin selection to advance between calls")
	}
}

// TestClientConnWait verifies that wait blocks while hung and reacts to reconnect and close.
func TestClientConnWait(t *testing.T) {
	cli := &Client{opts: &ClientOptions{WriteQueueSize: 8, WriteTimeout: time.Second}}

	c := newClientConn(cli)
	c.state.Store(connHanged)

	done := make(chan error, 1)
	go func() { done <- c.wait() }()

	select {
	case err := <-done:
		t.Fatalf("wait returned while the connection is hung: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	c.mu.Lock()
	c.state.Store(connOpened)
	c.cond.Broadcast()
	c.mu.Unlock()

	if err := <-done; err != nil {
		t.Errorf("expect wait to succeed after reconnect, got %v", err)
	}

	c2 := newClientConn(cli)
	c2.state.Store(connHanged)
	go func() { done <- c2.wait() }()
	time.Sleep(10 * time.Millisecond)

	c2.mu.Lock()
	c2.state.Store(connClosed)
	c2.cond.Broadcast()
	c2.mu.Unlock()

	if err := <-done; !errors.Is(err, errors.ErrConnectionClosed) {
		t.Errorf("expect %v after close, got %v", errors.ErrConnectionClosed, err)
	}
}

// TestClientConnClose verifies that close is idempotent and wakes pending calls.
func TestClientConnClose(t *testing.T) {
	_, c := newIdleClientConn()

	// The connection starts closed, so the first call is a no-op.
	c.close()

	c.state.Store(connOpened)
	ch := make(chan *buffer.Bytes, 1)
	c.pending.store(1, ch)

	c.close()

	if c.state.Load() != connClosed {
		t.Error("expect the connection to be marked closed")
	}
	if _, ok := <-ch; ok {
		t.Error("expect pending calls to be woken up")
	}
}

// TestClientConnDiscard verifies that discard drains and releases a buffered response.
func TestClientConnDiscard(t *testing.T) {
	_, c := newIdleClientConn()

	ch := make(chan *buffer.Bytes, 1)
	c.pending.store(9, ch)
	ch <- buffer.NewBytes([]byte("payload"))

	c.discard(9, ch)

	if _, ok := <-ch; ok {
		t.Error("expect the buffered response to be drained")
	}

	// An empty channel takes the default branch.
	empty := make(chan *buffer.Bytes, 1)
	c.pending.store(10, empty)
	c.discard(10, empty)
}

// TestClientConnCallHanged verifies that a call reports ErrConnectionHanged when pending calls are
// closed underneath it.
func TestClientConnCallHanged(t *testing.T) {
	_, c := newIdleClientConn()
	c.state.Store(connOpened)

	go func() {
		time.Sleep(20 * time.Millisecond)
		c.pending.closeAll()
	}()

	if _, err := c.call(context.Background(), 55, protocol.EncodeGetStateReq(55)); !errors.Is(err, errors.ErrConnectionHanged) {
		t.Errorf("expect %v, got %v", errors.ErrConnectionHanged, err)
	}
}

// TestClientConnDoPushClosed verifies that a closed client rejects pushes.
func TestClientConnDoPushClosed(t *testing.T) {
	_, c := newIdleClientConn()
	c.closed.Store(true)

	if err := c.doPush(protocol.EncodeGetStateReq(1)); !errors.Is(err, errors.ErrClientClosed) {
		t.Errorf("expect %v, got %v", errors.ErrClientClosed, err)
	}
}

// TestClientConnDoDialFailure verifies that dial gives up after the configured retries and closes
// the connection.
func TestClientConnDoDialFailure(t *testing.T) {
	cli, err := NewClient(listenFreeAddr(t), &ClientOptions{
		ConnNum:        1,
		DialTimeout:    100 * time.Millisecond,
		DialRetryTimes: 0,
		WriteQueueSize: 8,
		WriteTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}

	c := newClientConn(cli)
	c.state.Store(connOpened)

	if err = c.doDial(); err == nil {
		t.Fatal("expect dial to fail against an unused address")
	}
	if c.state.Load() != connClosed {
		t.Error("expect the connection to be closed after the retries are exhausted")
	}
}

// TestClientConnRetryIgnored verifies the guard branches of retry.
func TestClientConnRetryIgnored(t *testing.T) {
	_, c := newIdleClientConn()

	// A foreign session must be ignored.
	c.retry(&session{})
	if c.state.Load() != connClosed {
		t.Error("expect the connection state to stay unchanged")
	}

	// A closed client must be ignored.
	c.closed.Store(true)
	c.retry(nil)
}

// TestServerConnCheckState verifies the error mapped to each connection state.
func TestServerConnCheckState(t *testing.T) {
	tests := []struct {
		name  string
		state int32
		want  error
	}{
		{"opened", connOpened, errors.ErrConnectionNotAlived},
		{"hanged", connHanged, errors.ErrConnectionHanged},
		{"closed", connClosed, errors.ErrConnectionClosed},
		{"alived", connAlived, nil},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			conn := &ServerConn{}
			conn.state.Store(c.state)

			if err := conn.checkState(); !errors.Is(err, c.want) {
				t.Errorf("checkState(state=%d) = %v, want %v", c.state, err, c.want)
			}
		})
	}
}

// TestServerConnHandshakeInfo verifies the handshake information accessor.
func TestServerConnHandshakeInfo(t *testing.T) {
	conn := &ServerConn{}
	conn.kind, conn.inst = cluster.Node, "node-1"

	kind, inst := conn.HandshakeInfo()
	if kind != cluster.Node || inst != "node-1" {
		t.Errorf("HandshakeInfo() = (%v, %q), want (%v, %q)", kind, inst, cluster.Node, "node-1")
	}
}

// TestServerConnPush verifies that Push respects the connection state and the write queue.
func TestServerConnPush(t *testing.T) {
	t.Run("not alived", func(t *testing.T) {
		conn := &ServerConn{}
		conn.state.Store(connOpened)

		if err := conn.Push(protocol.EncodeGetStateRes(1, codes.OK, cluster.Work)); !errors.Is(err, errors.ErrConnectionNotAlived) {
			t.Errorf("expect %v, got %v", errors.ErrConnectionNotAlived, err)
		}
	})

	t.Run("alived", func(t *testing.T) {
		conn := &ServerConn{queue: queue.NewQueue[buffer.Buffer](4, time.Second)}
		conn.state.Store(connAlived)

		if err := conn.Push(protocol.EncodeGetStateRes(1, codes.OK, cluster.Work)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("closed queue", func(t *testing.T) {
		q := queue.NewQueue[buffer.Buffer](4, time.Second)
		q.Close()

		conn := &ServerConn{queue: q}
		conn.state.Store(connAlived)

		if err := conn.Push(protocol.EncodeGetStateRes(1, codes.OK, cluster.Work)); err == nil {
			t.Error("expect an error when the write queue is closed")
		}
	})
}

// TestServerConnGraceClose verifies the graceful close sequence.
func TestServerConnGraceClose(t *testing.T) {
	client, server := newTCPPair(t)
	svr := &Server{opts: &ServerOptions{WriteQueueSize: 16, WriteTimeout: time.Second}}
	conn := newServerConn(svr, server)

	if err := conn.graceClose(); err != nil {
		t.Fatalf("graceClose failed: %v", err)
	}
	if conn.state.Load() != connClosed {
		t.Error("expect the connection to be closed")
	}
	if err := conn.graceClose(); !errors.Is(err, errors.ErrConnectionNotOpened) {
		t.Errorf("second graceClose: err = %v, want %v", err, errors.ErrConnectionNotOpened)
	}

	_ = client.Close()
}

// TestServerConnCheckHeartbeat verifies the heartbeat timeout check.
func TestServerConnCheckHeartbeat(t *testing.T) {
	now := time.Now()

	expired := &ServerConn{}
	expired.lastHeartbeatTime.Store(now.Add(-3 * heartbeatInterval).UnixNano())
	// The connection is closed, so the scheduled force close is a no-op.
	expired.checkHeartbeat(&now)

	fresh := &ServerConn{}
	fresh.lastHeartbeatTime.Store(now.UnixNano())
	fresh.checkHeartbeat(&now)

	if fresh.state.Load() != connClosed {
		t.Error("a fresh connection must not be force-closed")
	}
}
