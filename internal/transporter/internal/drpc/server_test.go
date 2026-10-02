package drpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/petermattis/goid"
)

// listenFreeAddr returns an idle local listen address.
func listenFreeAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

// TestHandleMessageDispatchPolicy verifies the handler dispatch policy: enqueue-style uplink
// handlers (Deliver and Trigger) run synchronously on the read goroutine, while management
// handlers run asynchronously through the task pool.
func TestHandleMessageDispatchPolicy(t *testing.T) {
	s := &Server{}

	var (
		mu      sync.Mutex
		results = make(map[uint8]int64)
	)

	record := func(rt uint8) RouteHandler {
		return func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error {
			buf.Release()

			mu.Lock()
			results[rt] = goid.Get()
			mu.Unlock()

			return nil
		}
	}

	s.RegisterHandler(route.Deliver, record(route.Deliver))
	s.RegisterHandler(route.Trigger, record(route.Trigger))
	s.RegisterHandler(route.GetState, record(route.GetState))

	callerGoid := goid.Get()

	for _, rt := range []uint8{route.Deliver, route.Trigger, route.GetState} {
		if err := s.handleMessage(nil, rt, 0, buffer.NewBytes([]byte("x"))); err != nil {
			t.Fatalf("handleMessage failed: %v", err)
		}
	}

	if got := results[route.Deliver]; got != callerGoid {
		t.Fatalf("deliver should execute on the read goroutine, caller: %d, got: %d", callerGoid, got)
	}
	if got := results[route.Trigger]; got != callerGoid {
		t.Fatalf("trigger should execute on the read goroutine, caller: %d, got: %d", callerGoid, got)
	}

	// Management handlers run asynchronously on the task pool and must not occupy the read
	// goroutine.
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		got, ok := results[route.GetState]
		mu.Unlock()

		if ok {
			if got == callerGoid {
				t.Fatalf("getState should execute async on the task pool, got the read goroutine: %d", got)
			}
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("getState handler was not executed in time")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// TestServerDeliverOrdering sends multiple Deliver messages over one connection and requires the
// read goroutine to process them synchronously in send order.
func TestServerDeliverOrdering(t *testing.T) {
	const total = 100

	var (
		mu   sync.Mutex
		cids = make([]int64, 0, total)
	)

	addr := listenFreeAddr(t)

	s, err := NewServer(&ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	s.RegisterHandler(route.Deliver, func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error {
		cid, _, _, err := protocol.DecodeDeliverReq(buf)
		buf.Release()
		if err != nil {
			return err
		}

		mu.Lock()
		cids = append(cids, cid)
		mu.Unlock()

		return nil
	})

	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer s.Stop()

	cli, err := NewClient(addr, &ClientOptions{
		ID:             "test-gate",
		Kind:           cluster.Gate,
		ConnNum:        1,
		DialTimeout:    time.Second,
		DialRetryTimes: 3,
		WriteQueueSize: 4096,
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if err = cli.Establish(); err != nil {
		t.Fatalf("establish connection failed: %v", err)
	}
	defer cli.Close()

	for i := 0; i < total; i++ {
		if err = cli.Push(context.Background(), protocol.EncodeDeliverReq(0, int64(i), 0, buffer.NewBytes(nil)), int64(i)); err != nil {
			t.Fatalf("push failed: %v", err)
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(cids)
		mu.Unlock()

		if n == total {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("expect %d messages, got %d", total, n)
		}

		time.Sleep(10 * time.Millisecond)
	}

	for i := 0; i < total; i++ {
		if cids[i] != int64(i) {
			t.Fatalf("message out of order at %d: expect %d, got %d", i, i, cids[i])
		}
	}
}

// TestServerDeliverBackpressure verifies that the read goroutine stalls when a handler blocks:
// later messages are neither lost nor processed concurrently, and after release they are handled
// in order.
func TestServerDeliverBackpressure(t *testing.T) {
	release := make(chan struct{})
	processed := make(chan int64, 2)

	addr := listenFreeAddr(t)

	s, err := NewServer(&ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	s.RegisterHandler(route.Deliver, func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error {
		cid, _, _, err := protocol.DecodeDeliverReq(buf)
		buf.Release()
		if err != nil {
			return err
		}

		// Block until released, simulating the backpressure produced by a full node message queue.
		<-release

		processed <- cid

		return nil
	})

	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer s.Stop()

	cli, err := NewClient(addr, &ClientOptions{
		ID:             "test-gate",
		Kind:           cluster.Gate,
		ConnNum:        1,
		DialTimeout:    time.Second,
		DialRetryTimes: 3,
		WriteQueueSize: 4096,
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if err = cli.Establish(); err != nil {
		t.Fatalf("establish connection failed: %v", err)
	}
	defer cli.Close()

	for _, cid := range []int64{1, 2} {
		if err = cli.Push(context.Background(), protocol.EncodeDeliverReq(0, cid, 0, buffer.NewBytes(nil)), cid); err != nil {
			t.Fatalf("push failed: %v", err)
		}
	}

	// While the first message is blocked, the second message must not race ahead.
	select {
	case cid := <-processed:
		t.Fatalf("message %d should not be processed while the first is blocked", cid)
	case <-time.After(300 * time.Millisecond):
	}

	close(release)

	// After release, both messages are processed in order without loss.
	for _, want := range []int64{1, 2} {
		select {
		case cid := <-processed:
			if cid != want {
				t.Fatalf("message out of order: expect %d, got %d", want, cid)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("message lost under backpressure")
		}
	}
}
