package node

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
)

// newWorkingEvent creates a node in the work state together with an event context bound to it.
func newWorkingEvent(t *testing.T, opts ...Option) (*Node, *event) {
	t.Helper()

	n := newTestNode(t, opts...)
	setWorking(n)

	e := n.evtPool.Get().(*event)
	e.ctx = context.Background()

	return n, e
}

// TestEventGetters verifies the event accessors.
func TestEventGetters(t *testing.T) {
	e := &event{gid: "gate-1", cid: 11, uid: 22, event: cluster.Connect}

	if e.GID() != "gate-1" || e.NID() != "" || e.CID() != 11 || e.UID() != 22 {
		t.Fatalf("unexpected getters: %+v", e)
	}
	if e.Seq() != 0 || e.Route() != 0 {
		t.Fatalf("event seq/route should be zero, got %d/%d", e.Seq(), e.Route())
	}
	if e.Event() != cluster.Connect {
		t.Fatalf("unexpected event: %v", e.Event())
	}
	if e.Kind() != Event {
		t.Fatalf("expect event kind, got %v", e.Kind())
	}
}

// TestEventParseAndResponse verifies the unsupported operations of an event.
func TestEventParseAndResponse(t *testing.T) {
	e := &event{}

	if err := e.Parse(struct{}{}); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}
	if err := e.Response("payload"); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}
}

// TestEventDeferAndCancel verifies the defer call stack of an event.
func TestEventDeferAndCancel(t *testing.T) {
	e := &event{}

	var order []int
	e.Defer(func() { order = append(order, 1) })
	e.Defer(func() { order = append(order, 2) })
	e.Defer(func() { order = append(order, 3) }, true)

	e.compareVersionExecDefer(e.loadVersion())

	if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 3 {
		t.Fatalf("unexpected defer order: %v", order)
	}

	var called atomic.Int32
	e2 := &event{}
	e2.Defer(func() { called.Add(1) })
	e2.Cancel()
	e2.compareVersionExecDefer(e2.loadVersion())

	if called.Load() != 0 {
		t.Fatal("cancelled defer should not run")
	}
}

// TestEventClone verifies cloning an event context.
func TestEventClone(t *testing.T) {
	n, e := newWorkingEvent(t)
	e.gid = "gate-1"
	e.cid = 5
	e.uid = 6
	e.event = cluster.Reconnect

	act := &Actor{}
	e.actor.Store(act)

	c := e.Clone().(*event)
	if c.gid != "gate-1" || c.cid != 5 || c.uid != 6 || c.event != cluster.Reconnect {
		t.Fatalf("unexpected clone: %+v", c)
	}
	if c.actor.Load() != act {
		t.Fatal("the actor should be carried over to the clone")
	}
	if c.ctx != e.ctx {
		t.Fatal("the context should be carried over to the clone")
	}

	_ = n
}

// TestEventTask verifies submitting an asynchronous event task.
func TestEventTask(t *testing.T) {
	t.Run("working node", func(t *testing.T) {
		_, e := newWorkingEvent(t)

		done := make(chan struct{})
		e.Task(func(ctx Context) { close(done) })

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("task should be executed")
		}
	})

	t.Run("shut node", func(t *testing.T) {
		n := newTestNode(t)
		e := n.evtPool.Get().(*event)

		var called atomic.Int32
		e.Task(func(ctx Context) { called.Add(1) })

		time.Sleep(50 * time.Millisecond)
		if called.Load() != 0 {
			t.Fatal("task on a shut node should not run")
		}
	})
}

// TestEventNext verifies dispatching an event with no schedulable actors.
func TestEventNext(t *testing.T) {
	_, e := newWorkingEvent(t)

	if err := e.Next(); err != nil {
		t.Fatalf("next should succeed with no actors, got %v", err)
	}
}

// TestEventProxyAndContext verifies the proxy and context helpers.
func TestEventProxyAndContext(t *testing.T) {
	n, e := newWorkingEvent(t)

	if e.Proxy() != n.proxy {
		t.Fatal("proxy mismatch")
	}
	if e.Context() != e.ctx {
		t.Fatal("context mismatch")
	}

	type key struct{}
	e.SetValue(key{}, "v")
	if got := e.GetValue(key{}); got != "v" {
		t.Fatalf("expect value v, got %v", got)
	}
}

// TestEventBindGate verifies the gateway binding paths of an event.
func TestEventBindGate(t *testing.T) {
	t.Run("explicit uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.gid = "gate-1"
		e.cid = 1

		if err := e.BindGate(100); err == nil {
			t.Fatal("expect a binding error")
		}
	})

	t.Run("current uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.uid = 100

		if err := e.BindGate(); err == nil {
			t.Fatal("expect a binding error")
		}
	})

	t.Run("missing uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.BindGate(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("unbind explicit uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.UnbindGate(100); err == nil {
			t.Fatal("expect an unbinding error")
		}
	})

	t.Run("unbind current uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.uid = 100
		if err := e.UnbindGate(); err == nil {
			t.Fatal("expect an unbinding error")
		}
	})

	t.Run("unbind missing uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.UnbindGate(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestEventBindNode verifies the node binding paths of an event.
func TestEventBindNode(t *testing.T) {
	t.Run("explicit uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.BindNode(100); err != nil {
			t.Fatalf("bind node failed: %v", err)
		}
	})

	t.Run("current uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.uid = 100
		if err := e.BindNode(); err != nil {
			t.Fatalf("bind node failed: %v", err)
		}
	})

	t.Run("missing uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.BindNode(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("unbind explicit uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.UnbindNode(100); err != nil {
			t.Fatalf("unbind node failed: %v", err)
		}
	})

	t.Run("unbind current uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.uid = 100
		if err := e.UnbindNode(); err != nil {
			t.Fatalf("unbind node failed: %v", err)
		}
	})

	t.Run("unbind missing uid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if err := e.UnbindNode(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestEventSubscribe verifies the subscribe and unsubscribe paths of an event.
func TestEventSubscribe(t *testing.T) {
	t.Run("user targets", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.gid = "gate-1"

		_ = e.Subscribe("ch", 1, 2)
		_ = e.Unsubscribe("ch", 1, 2)
	})

	t.Run("conn target", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.gid = "gate-1"
		e.cid = 3

		_ = e.Subscribe("ch")
		_ = e.Unsubscribe("ch")
	})

	t.Run("missing gid", func(t *testing.T) {
		_, e := newWorkingEvent(t)

		if err := e.Subscribe("ch"); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
		if err := e.Unsubscribe("ch"); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestEventActorHelpers verifies the actor helpers of an event.
func TestEventActorHelpers(t *testing.T) {
	n, e := newWorkingEvent(t)
	registerTestActor(n.scheduler, "room", "1")
	e.uid = 100

	if err := e.BindActor("room", "1"); err != nil {
		t.Fatalf("bind actor failed: %v", err)
	}
	if _, ok := e.Actor("room", "1"); !ok {
		t.Fatal("actor should exist")
	}

	e.UnbindActor("room")

	if _, err := e.Spawn(nil); !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("expect ErrInvalidArgument, got %v", err)
	}
	if e.Kill("not-exist", "1") {
		t.Fatal("killing a missing actor should return false")
	}
}

// TestEventInvoke verifies the invoke helper for both the global and the actor scope.
func TestEventInvoke(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		_, e := newWorkingEvent(t)

		if err := e.Invoke(func() {}); err != nil {
			t.Fatalf("invoke failed: %v", err)
		}
	})

	t.Run("actor", func(t *testing.T) {
		n, e := newWorkingEvent(t)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		e.actor.Store(act)

		done := make(chan struct{})
		if err = e.Invoke(func() { close(done) }, true); err != nil {
			t.Fatalf("invoke failed: %v", err)
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("actor invoke should be executed")
		}
	})
}

// TestEventAfterFunc verifies the delayed call helpers of an event.
func TestEventAfterFunc(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		_, e := newWorkingEvent(t)

		done := make(chan struct{})
		if _, err := e.AfterFunc(10*time.Millisecond, func() { close(done) }); err != nil {
			t.Fatalf("after func failed: %v", err)
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("after func should run")
		}

		if _, err := e.AfterInvoke(time.Millisecond, func() {}); err != nil {
			t.Fatalf("after invoke failed: %v", err)
		}
	})

	t.Run("actor", func(t *testing.T) {
		n, e := newWorkingEvent(t)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("2"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		e.actor.Store(act)

		if _, err = e.AfterFunc(5*time.Millisecond, func() {}); err != nil {
			t.Fatalf("actor after func failed: %v", err)
		}
		if _, err = e.AfterInvoke(5*time.Millisecond, func() {}); err != nil {
			t.Fatalf("actor after invoke failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)
	})
}

// TestEventGetIP verifies the client IP helper of an event.
func TestEventGetIP(t *testing.T) {
	t.Run("missing gid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if _, err := e.GetIP(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("with gid", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.gid = "gate-1"

		if _, err := e.GetIP(); err == nil {
			t.Fatal("expect an error without a gate endpoint")
		}
	})
}

// TestEventDeliver verifies the deliver helper of an event.
func TestEventDeliver(t *testing.T) {
	_, e := newWorkingEvent(t)

	if err := e.Deliver(&cluster.DeliverArgs{NID: "other", Message: &cluster.Message{Route: 1}}); err == nil {
		t.Fatal("expect an error without a node endpoint")
	}
}

// TestEventReply verifies the reply paths of an event.
func TestEventReply(t *testing.T) {
	t.Run("user target", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.gid = "gate-1"
		e.uid = 100

		if err := e.Reply(&cluster.Message{Route: 1}); err == nil {
			t.Fatal("expect an error without a gate endpoint")
		}
	})

	t.Run("conn target", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		e.gid = "gate-1"
		e.cid = 1

		if err := e.Reply(&cluster.Message{Route: 1}); err == nil {
			t.Fatal("expect an error without a gate endpoint")
		}
	})
}

// TestEventDisconnect verifies the disconnect helper of an event.
func TestEventDisconnect(t *testing.T) {
	_, e := newWorkingEvent(t)
	e.gid = "gate-1"

	if err := e.Disconnect(true); err == nil {
		t.Fatal("expect an error without a gate endpoint")
	}
}

// TestEventNewMeshClient verifies the mesh client helper of an event.
func TestEventNewMeshClient(t *testing.T) {
	t.Run("missing transporter", func(t *testing.T) {
		_, e := newWorkingEvent(t)
		if _, err := e.NewMeshClient("direct://127.0.0.1:1"); !errors.Is(err, errors.ErrMissingTransporter) {
			t.Fatalf("expect ErrMissingTransporter, got %v", err)
		}
	})

	t.Run("with transporter", func(t *testing.T) {
		_, e := newWorkingEvent(t, WithTransporter(&stubTransporter{}))
		if _, err := e.NewMeshClient("direct://127.0.0.1:1"); err == nil {
			t.Fatal("expect an error from the stubbed transporter")
		}
	})
}

// TestEventStateHelpers verifies the actor, version and defer helpers of an event.
func TestEventStateHelpers(t *testing.T) {
	e := &event{}
	act := &Actor{}

	e.storeActor(act)
	if e.actor.Load() != act {
		t.Fatal("actor should be stored")
	}
	e.deleteActor()
	if e.actor.Load() != nil {
		t.Fatal("actor should be cleared")
	}

	if e.loadVersion() != 0 {
		t.Fatal("initial version should be zero")
	}
	if v := e.incrVersion(); v != 1 {
		t.Fatalf("expect version 1, got %d", v)
	}
	if v := e.decrVersion(); v != 0 {
		t.Fatalf("expect version 0, got %d", v)
	}

	e.Defer(func() {})
	e.cancelDefer()
	e.recoverDefer()
	e.releaseDefer()
}

// TestEventCompareVersionRecycle verifies the version-based recycling of an event.
func TestEventCompareVersionRecycle(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	e := n.evtPool.Get().(*event)
	e.version.Store(5)
	e.compareVersionRecycle(5)

	if e.event != 0 || e.uid != 0 {
		t.Fatal("event should be recycled")
	}

	e2 := n.evtPool.Get().(*event)
	e2.version.Store(7)
	e2.compareVersionRecycle(8)

	if e2.version.Load() != 7 {
		t.Fatal("a mismatched version should not recycle")
	}
}

// TestEventRelease verifies releasing an event.
func TestEventRelease(t *testing.T) {
	n := newTestNode(t)

	e := &event{node: n, ctx: context.Background(), gid: "gate-1", cid: 1, uid: 2, event: cluster.Connect}
	e.Defer(func() {})
	e.release()

	if e.gid != "" || e.cid != 0 || e.uid != 0 || e.event != 0 || e.chain != nil {
		t.Fatalf("event should be fully reset: %+v", e)
	}
	if e.ctx != context.Background() {
		t.Fatal("context should be reset to background")
	}
}

// TestEventInvokeOnShutNode verifies the invoke and delayed helpers on a shut node.
func TestEventInvokeOnShutNode(t *testing.T) {
	n := newTestNode(t)
	e := n.evtPool.Get().(*event)

	if err := e.Invoke(func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
		t.Fatalf("expect ErrNodeShutdown, got %v", err)
	}
	if _, err := e.AfterFunc(time.Millisecond, func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
		t.Fatalf("expect ErrNodeShutdown, got %v", err)
	}
	if _, err := e.AfterInvoke(time.Millisecond, func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
		t.Fatalf("expect ErrNodeShutdown, got %v", err)
	}
}
