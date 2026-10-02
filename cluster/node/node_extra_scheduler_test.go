package node

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
)

// TestSchedulerDispatchRequest verifies the request dispatch paths.
func TestSchedulerDispatchRequest(t *testing.T) {
	t.Run("missing uid", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		req := n.reqPool.Get().(*request)
		if err := n.scheduler.dispatch(req); !errors.Is(err, errors.ErrMissingDispatchStrategy) {
			t.Fatalf("expect ErrMissingDispatchStrategy, got %v", err)
		}
	})

	t.Run("unregistered route", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		req := n.reqPool.Get().(*request)
		req.uid = 100
		req.route = 404

		if err := n.scheduler.dispatch(req); !errors.Is(err, errors.ErrUnregisterRoute) {
			t.Fatalf("expect ErrUnregisterRoute, got %v", err)
		}
	})

	t.Run("unbound user", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		n.scheduler.routes.Store(int32(1), "room")

		req := n.reqPool.Get().(*request)
		req.uid = 100
		req.route = 1

		if err := n.scheduler.dispatch(req); !errors.Is(err, errors.ErrNotBindActor) {
			t.Fatalf("expect ErrNotBindActor, got %v", err)
		}
	})

	t.Run("bound actor", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		var called atomic.Int32

		// Register the route through the actor task queue and wait until it has been applied, so
		// that the route map is never accessed concurrently from the test goroutine.
		registered := make(chan struct{})
		act.AddRouteHandler(1, func(ctx Context) { called.Add(1) })
		if _, err = act.taskQueue.Commit(func() { close(registered) }); err != nil {
			t.Fatalf("commit failed: %v", err)
		}

		select {
		case <-registered:
		case <-time.After(2 * time.Second):
			t.Fatal("the route handler should be registered")
		}

		n.scheduler.routes.Store(int32(1), "room")
		if err = n.scheduler.bindActor(100, "room", "1"); err != nil {
			t.Fatalf("bind actor failed: %v", err)
		}

		req := n.reqPool.Get().(*request)
		req.ctx = newTestNodeCtx()
		req.uid = 100
		req.route = 1
		req.message = []byte("x")

		if err = n.scheduler.dispatch(req); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		deadline := time.Now().Add(2 * time.Second)
		for called.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if called.Load() != 1 {
			t.Fatalf("expect the actor handler called once, got %d", called.Load())
		}
	})
}

// TestSchedulerDispatchEvent verifies the event dispatch paths.
func TestSchedulerDispatchEvent(t *testing.T) {
	t.Run("no actor", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		e := n.evtPool.Get().(*event)
		if err := n.scheduler.dispatch(e); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}
	})

	t.Run("matching actor", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		var called atomic.Int32
		act.events.Store(cluster.Connect, EventHandler(func(ctx Context) { called.Add(1) }))

		n.scheduler.routes.Store(int32(1), "room")
		if err = n.scheduler.bindActor(100, "room", "1"); err != nil {
			t.Fatalf("bind actor failed: %v", err)
		}

		e := n.evtPool.Get().(*event)
		e.ctx = newTestNodeCtx()
		e.event = cluster.Connect
		e.uid = 100

		if err = n.scheduler.dispatch(e); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		deadline := time.Now().Add(2 * time.Second)
		for called.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if called.Load() != 1 {
			t.Fatalf("expect the actor event handler called once, got %d", called.Load())
		}
	})

	t.Run("non dispatchable actor", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("2"), WithActorNonDispatch())
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		e := n.evtPool.Get().(*event)
		e.event = cluster.Connect

		if err = n.scheduler.dispatch(e); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}
	})
}

// TestSchedulerSpawnErrors verifies the actor creation error paths.
func TestSchedulerSpawnErrors(t *testing.T) {
	t.Run("invalid arguments", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		if _, err := n.scheduler.spawn(newStubProcessorCreator()); !errors.Is(err, errors.ErrInvalidArgument) {
			t.Fatalf("expect ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("nil processor", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		_, err := n.scheduler.spawn(func(actor *Actor, args ...any) Processor { return nil },
			WithActorKind("room"), WithActorID("1"))
		if !errors.Is(err, errors.ErrActorCreateFailed) {
			t.Fatalf("expect ErrActorCreateFailed, got %v", err)
		}
	})

	t.Run("duplicate actor", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		if _, err = n.scheduler.spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1")); !errors.Is(err, errors.ErrActorExists) {
			t.Fatalf("expect ErrActorExists, got %v", err)
		}
	})

	t.Run("node shutdown", func(t *testing.T) {
		n := newTestNode(t)

		_, err := n.scheduler.spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})

	t.Run("non waited actor on shut node", func(t *testing.T) {
		n := newTestNode(t)

		// WithActorNonWait disables the wait registration, so the actor may still be created.
		act, err := n.scheduler.spawn(newStubProcessorCreator(),
			WithActorKind("room"), WithActorID("1"), WithActorNonWait())
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })
	})
}

// TestSchedulerKill verifies killing an actor.
func TestSchedulerKill(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	if !n.scheduler.kill("room", "1") {
		t.Fatal("expect the actor to be killed")
	}
	if n.scheduler.kill("room", "1") {
		t.Fatal("expect the second kill to fail")
	}

	_ = act
}

// TestSchedulerReleaseKind verifies releasing a non-existent kind.
func TestSchedulerReleaseKind(t *testing.T) {
	n := newTestNode(t)
	n.scheduler.releaseKind("missing")
}
