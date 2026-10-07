package node

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
)

// newUnstartActor creates an actor that has not been started, used to exercise the behavior of
// the actor methods on a non-started actor.
func newUnstartActor() *Actor {
	return &Actor{
		opts:   &actorOptions{kind: "room", id: "1"},
		pid:    "room/1",
		rw:     &sync.RWMutex{},
		routes: make(map[int32]RouteHandler),
	}
}

// waitActorReady waits until fn reports true or the timeout elapses.
func waitActorReady(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatal("timed out waiting for the actor to be ready")
}

// TestActorAccessors verifies the actor accessors.
func TestActorAccessors(t *testing.T) {
	act := newUnstartActor()

	if act.ID() != "1" || act.Kind() != "room" || act.PID() != "room/1" {
		t.Fatalf("unexpected accessors: %s/%s/%s", act.ID(), act.Kind(), act.PID())
	}
	if act.started() {
		t.Fatal("the actor should not be started")
	}
}

// TestActorUnstartMethods verifies that the configuration methods are no-ops on a non-started
// actor.
func TestActorUnstartMethods(t *testing.T) {
	act := newUnstartActor()

	act.SetDefaultRouteHandler(func(ctx Context) {})
	if act.defaultRouteHandler != nil {
		t.Fatal("the default route handler should not be set on a non-started actor")
	}

	act.AddRouteHandler(1, func(ctx Context) {})
	if _, ok := act.routes[1]; ok {
		t.Fatal("route 1 should not be registered on a non-started actor")
	}

	act.AddEventHandler(cluster.Connect, func(ctx Context) {})
	if _, ok := act.events.Load(cluster.Connect); ok {
		t.Fatal("the event handler should not be registered on a non-started actor")
	}

	if err := act.Invoke(func() {}); !errors.Is(err, errors.ErrActorNotStarted) {
		t.Fatalf("expect ErrActorNotStarted, got %v", err)
	}
	if _, err := act.AfterFunc(time.Millisecond, func() {}); !errors.Is(err, errors.ErrActorNotStarted) {
		t.Fatalf("expect ErrActorNotStarted, got %v", err)
	}
	if _, err := act.AfterInvoke(time.Millisecond, func() {}); !errors.Is(err, errors.ErrActorNotStarted) {
		t.Fatalf("expect ErrActorNotStarted, got %v", err)
	}

	req := &request{node: &Node{}}
	if err := act.Next(req); !errors.Is(err, errors.ErrActorNotStarted) {
		t.Fatalf("expect ErrActorNotStarted, got %v", err)
	}
}

// TestActorProxyAndSpawn verifies the proxy accessor and the spawn delegation.
func TestActorProxyAndSpawn(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	t.Cleanup(func() { act.Destroy() })

	if act.Proxy() != n.proxy {
		t.Fatal("proxy mismatch")
	}

	child, err := act.Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("2"))
	if err != nil {
		t.Fatalf("spawn child failed: %v", err)
	}
	t.Cleanup(func() { child.Destroy() })

	if _, ok := n.Proxy().Actor("room", "2"); !ok {
		t.Fatal("the child actor should be registered")
	}
}

// TestActorDestroy verifies the actor destruction paths.
func TestActorDestroy(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	proc := &stubProcessor{}
	act, err := n.Proxy().Spawn(func(actor *Actor, args ...any) Processor { return proc },
		WithActorKind("room"), WithActorID("1"))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	waitActorReady(t, func() bool { return proc.startCount.Load() == 1 })

	if !act.Destroy() {
		t.Fatal("expect the actor to be destroyed")
	}
	if act.Destroy() {
		t.Fatal("the second destroy should report false")
	}

	waitActorReady(t, func() bool { return proc.destroyCount.Load() == 1 })
	if proc.initCount.Load() != 1 {
		t.Fatalf("expect the processor init once, got %d", proc.initCount.Load())
	}
}

// TestActorInvokeAndDelayed verifies the thread-safe and delayed call helpers.
func TestActorInvokeAndDelayed(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	t.Cleanup(func() { act.Destroy() })

	done := make(chan struct{})
	timer, err := act.AfterFunc(5*time.Millisecond, func() { close(done) })
	if err != nil {
		t.Fatalf("after func failed: %v", err)
	}
	if timer == nil {
		t.Fatal("timer should not be nil")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the actor after func should run")
	}

	if _, err = act.AfterInvoke(5*time.Millisecond, func() {}); err != nil {
		t.Fatalf("after invoke failed: %v", err)
	}

	invoked := make(chan struct{})
	if err = act.Invoke(func() { close(invoked) }, true); err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	select {
	case <-invoked:
	case <-time.After(2 * time.Second):
		t.Fatal("the actor invoke should run")
	}

	time.Sleep(30 * time.Millisecond)
}

// TestActorStartedHandlers verifies registering handlers on a started actor.
func TestActorStartedHandlers(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	t.Cleanup(func() { act.Destroy() })

	// Registering a route on a dispatchable actor also updates the scheduler route table.
	registered := make(chan struct{})
	act.AddRouteHandler(7, func(ctx Context) {})
	if _, err = act.taskQueue.Commit(func() { close(registered) }); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	<-registered

	if kind, ok := n.scheduler.routes.Load(int32(7)); !ok || kind.(string) != "room" {
		t.Fatalf("the scheduler route table should map 7 to room, got %v/%v", kind, ok)
	}

	// Registering an event handler on a started actor.
	handlerDone := make(chan struct{})
	act.AddEventHandler(cluster.Connect, func(ctx Context) { close(handlerDone) })
	waitActorReady(t, func() bool {
		_, ok := act.events.Load(cluster.Connect)
		return ok
	})

	// Setting a default route handler on a started actor.
	defaultReady := make(chan struct{})
	act.SetDefaultRouteHandler(func(ctx Context) {})
	if _, err = act.taskQueue.Commit(func() { close(defaultReady) }); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	<-defaultReady
}

// TestActorNextAndDispatch verifies message delivery and handler dispatch.
func TestActorNextAndDispatch(t *testing.T) {
	t.Run("route handler", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		var called atomic.Int32
		registered := make(chan struct{})
		act.AddRouteHandler(1, func(ctx Context) { called.Add(1) })
		if _, err = act.taskQueue.Commit(func() { close(registered) }); err != nil {
			t.Fatalf("commit failed: %v", err)
		}
		<-registered

		req := n.reqPool.Get().(*request)
		req.ctx = newTestNodeCtx()
		req.route = 1
		req.message = []byte("x")

		if err = act.Next(req); err != nil {
			t.Fatalf("next failed: %v", err)
		}

		waitActorReady(t, func() bool { return called.Load() == 1 })
	})

	t.Run("default route handler", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		ready := make(chan struct{})
		act.SetDefaultRouteHandler(func(ctx Context) {})
		if _, err = act.taskQueue.Commit(func() { close(ready) }); err != nil {
			t.Fatalf("commit failed: %v", err)
		}
		<-ready

		req := n.reqPool.Get().(*request)
		req.ctx = newTestNodeCtx()
		req.route = 999
		req.message = []byte("x")

		if err = act.Next(req); err != nil {
			t.Fatalf("next failed: %v", err)
		}

		time.Sleep(30 * time.Millisecond)
	})

	t.Run("event handler", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		var called atomic.Int32
		act.events.Store(cluster.Connect, EventHandler(func(ctx Context) { called.Add(1) }))

		evt := n.evtPool.Get().(*event)
		evt.ctx = newTestNodeCtx()
		evt.event = cluster.Connect

		if err = act.Next(evt); err != nil {
			t.Fatalf("next failed: %v", err)
		}

		waitActorReady(t, func() bool { return called.Load() == 1 })
	})

	t.Run("closed message queue", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		act.messageQueue.Close()

		req := n.reqPool.Get().(*request)
		req.ctx = newTestNodeCtx()

		if err = act.Next(req); err == nil {
			t.Fatal("expect an error when the message queue is closed")
		}
	})
}

// TestActorDeliverAndPush verifies the deliver and push helpers.
func TestActorDeliverAndPush(t *testing.T) {
	t.Run("deliver", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		var called atomic.Int32
		registered := make(chan struct{})
		act.AddRouteHandler(1, func(ctx Context) { called.Add(1) })
		if _, err = act.taskQueue.Commit(func() { close(registered) }); err != nil {
			t.Fatalf("commit failed: %v", err)
		}
		<-registered

		if err = act.Deliver(100, &cluster.Message{Route: 1, Data: []byte("x")}); err != nil {
			t.Fatalf("deliver failed: %v", err)
		}

		waitActorReady(t, func() bool { return called.Load() == 1 })
	})

	t.Run("deliver with custom context", func(t *testing.T) {
		n := newTestNode(t, WithContextFunc(newTestNodeCtx))
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		if err = act.Deliver(100, &cluster.Message{Route: 1, Data: []byte("x")}); err != nil {
			t.Fatalf("deliver failed: %v", err)
		}
	})

	t.Run("push", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		if err = act.Push(100, &cluster.Message{Route: 1, Data: []byte("x")}); err != nil {
			t.Fatalf("push failed: %v", err)
		}

		select {
		case req := <-n.router.receive():
			if req.uid != 100 || req.route != 1 {
				t.Fatalf("unexpected pushed request: %+v", req)
			}
			req.release()
		case <-time.After(time.Second):
			t.Fatal("expect a request in the routing queue")
		}
	})
}

// TestActorBinds verifies the user binding helpers.
func TestActorBinds(t *testing.T) {
	act := &Actor{}

	act.bindUser(100)
	if !act.unbindUser(100) {
		t.Fatal("expect the user to be unbound")
	}
	if act.unbindUser(100) {
		t.Fatal("the second unbind should report false")
	}
}

// TestActorInvokeOnNonStarted verifies the invoke helper rejects a non-started actor.
func TestActorInvokeOnNonStarted(t *testing.T) {
	act := newUnstartActor()

	if err := act.Invoke(func() {}); !errors.Is(err, errors.ErrActorNotStarted) {
		t.Fatalf("expect ErrActorNotStarted, got %v", err)
	}
}
