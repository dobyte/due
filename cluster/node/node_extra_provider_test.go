package node

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// packTestMessage packs a test message for the given route.
func packTestMessage(t *testing.T, route int32) buffer.Buffer {
	t.Helper()

	buf, err := packet.PackMessage(&packet.Message{
		Seq:    1,
		Route:  route,
		Buffer: []byte("payload"),
	})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	return buf
}

// TestProviderTrigger verifies the provider trigger paths.
func TestProviderTrigger(t *testing.T) {
	t.Run("shut node", func(t *testing.T) {
		n := newTestNode(t)
		p := &provider{node: n}

		if err := p.Trigger(context.Background(), "gate-1", 1, 2, cluster.Connect); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})

	t.Run("working node", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)
		p := &provider{node: n}

		if err := p.Trigger(context.Background(), "gate-1", 1, 2, cluster.Connect); err != nil {
			t.Fatalf("trigger failed: %v", err)
		}

		select {
		case evt := <-n.trigger.receive():
			if evt.event != cluster.Connect || evt.gid != "gate-1" || evt.cid != 1 || evt.uid != 2 {
				t.Fatalf("unexpected event: %+v", evt)
			}
			evt.release()
		case <-time.After(time.Second):
			t.Fatal("expect an event in the queue")
		}
	})
}

// TestProviderDeliver verifies the provider deliver paths.
func TestProviderDeliver(t *testing.T) {
	t.Run("shut node", func(t *testing.T) {
		n := newTestNode(t)
		p := &provider{node: n}

		buf := packTestMessage(t, 1)
		if err := p.Deliver(context.Background(), "gate-1", "", 1, 2, buf); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})

	t.Run("invalid buffer", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)
		p := &provider{node: n}

		if err := p.Deliver(context.Background(), "gate-1", "", 1, 2, buffer.NewBytes([]byte("ab"))); err == nil {
			t.Fatal("expect an unpack error")
		}
	})

	t.Run("unregistered route without default", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)
		p := &provider{node: n}

		if err := p.Deliver(context.Background(), "gate-1", "", 1, 2, packTestMessage(t, 404)); err != nil {
			t.Fatalf("deliver should be a no-op, got %v", err)
		}
	})

	t.Run("stateful route without uid", func(t *testing.T) {
		n := newTestNode(t)
		n.router.AddRouteHandler(1, func(ctx Context) {}, StatefulRoute)
		setWorking(n)
		p := &provider{node: n}

		if err := p.Deliver(context.Background(), "gate-1", "", 1, 0, packTestMessage(t, 1)); !errors.Is(err, errors.ErrInvalidArgument) {
			t.Fatalf("expect ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("stateful route with mismatched node", func(t *testing.T) {
		n, locator, _ := stubNode(t)
		n.router.AddRouteHandler(1, func(ctx Context) {}, StatefulRoute)
		locator.nid = "other-node"
		setWorking(n)
		p := &provider{node: n}

		if err := p.Deliver(context.Background(), "gate-1", "", 1, 100, packTestMessage(t, 1)); !errors.Is(err, errors.ErrNotFoundSession) {
			t.Fatalf("expect ErrNotFoundSession, got %v", err)
		}
	})

	t.Run("stateful route success", func(t *testing.T) {
		n, locator, _ := stubNode(t)
		n.router.AddRouteHandler(1, func(ctx Context) {}, StatefulRoute)
		locator.nid = n.opts.id
		setWorking(n)
		p := &provider{node: n}

		if err := p.Deliver(context.Background(), "gate-1", "node-2", 1, 100, packTestMessage(t, 1)); err != nil {
			t.Fatalf("deliver failed: %v", err)
		}

		select {
		case req := <-n.router.receive():
			if req.gid != "gate-1" || req.nid != "node-2" || req.uid != 100 {
				t.Fatalf("unexpected delivered request: %+v", req)
			}
			req.release()
		case <-time.After(time.Second):
			t.Fatal("expect a request in the routing queue")
		}
	})

	t.Run("stateless route", func(t *testing.T) {
		n := newTestNode(t)
		n.router.AddRouteHandler(1, func(ctx Context) {})
		setWorking(n)
		p := &provider{node: n}

		if err := p.Deliver(context.Background(), "gate-1", "", 1, 100, packTestMessage(t, 1)); err != nil {
			t.Fatalf("deliver failed: %v", err)
		}

		select {
		case req := <-n.router.receive():
			req.release()
		case <-time.After(time.Second):
			t.Fatal("expect a request in the routing queue")
		}
	})
}

// TestProviderState verifies the provider state accessors.
func TestProviderState(t *testing.T) {
	t.Run("get state", func(t *testing.T) {
		n := newTestNode(t)
		p := &provider{node: n}

		state, err := p.GetState()
		if err != nil || state != cluster.Shut {
			t.Fatalf("expect shut state and nil error, got %v/%v", state, err)
		}
	})

	t.Run("set state on shut node", func(t *testing.T) {
		n := newTestNode(t)
		p := &provider{node: n}

		if err := p.SetState(cluster.Work); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("set state on working node", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)
		p := &provider{node: n}

		if err := p.SetState(cluster.Busy); err != nil {
			t.Fatalf("set state failed: %v", err)
		}
		if got := n.getState(); got != cluster.Busy {
			t.Fatalf("expect busy state, got %v", got)
		}
	})
}
