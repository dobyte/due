package node

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestRouterAddRouteHandler verifies the route registration and the route state check.
func TestRouterAddRouteHandler(t *testing.T) {
	n := newTestNode(t)
	r := n.router

	r.AddRouteHandler(1, func(ctx Context) {})
	r.AddRouteHandler(2, func(ctx Context) {}, StatefulRoute)

	stateful, exist := r.CheckRouteStateful(1)
	if !exist || stateful {
		t.Fatalf("route 1 should exist and be stateless, exist=%v stateful=%v", exist, stateful)
	}

	stateful, exist = r.CheckRouteStateful(2)
	if !exist || !stateful {
		t.Fatalf("route 2 should exist and be stateful, exist=%v stateful=%v", exist, stateful)
	}

	if _, exist = r.CheckRouteStateful(99); exist {
		t.Fatal("route 99 should not exist")
	}

	// Registering while the node is not shut is rejected.
	setWorking(n)
	r.AddRouteHandler(3, func(ctx Context) {})
	if _, exist = r.CheckRouteStateful(3); exist {
		t.Fatal("route 3 should not be registered while working")
	}
}

// TestRouterHandlers verifies the default, pre and post route handlers.
func TestRouterHandlers(t *testing.T) {
	n := newTestNode(t)
	r := n.router

	if r.HasDefaultRouteHandler() {
		t.Fatal("default route handler should be nil")
	}

	r.SetDefaultRouteHandler(func(ctx Context) {})
	r.SetPreRouteHandler(func(ctx Context) {})
	r.SetPostRouteHandler(func(ctx Context) {})

	if !r.HasDefaultRouteHandler() {
		t.Fatal("default route handler should be set")
	}

	// Setting handlers while the node is not shut is rejected.
	setWorking(n)
	r.SetDefaultRouteHandler(nil)
	r.SetPreRouteHandler(nil)
	r.SetPostRouteHandler(nil)

	if !r.HasDefaultRouteHandler() {
		t.Fatal("default route handler should be kept while working")
	}
}

// TestRouterGroup verifies the route group middleware merging.
func TestRouterGroup(t *testing.T) {
	n := newTestNode(t)
	r := n.router

	var order []string

	mw1 := func(m *Middleware, ctx Context) {
		order = append(order, "mw1")
		m.Next(ctx)
	}
	mw2 := func(m *Middleware, ctx Context) {
		order = append(order, "mw2")
		m.Next(ctx)
	}
	mw3 := func(m *Middleware, ctx Context) {
		order = append(order, "mw3")
		m.Next(ctx)
	}

	group := r.Group(func(g *RouterGroup) {})
	group.Middleware(mw1).Middleware(mw2)
	group.AddRouteHandler(7, func(ctx Context) { order = append(order, "handler") }, RouteOptions{
		Middlewares: []MiddlewareHandler{mw3},
	})

	entity, ok := r.routes[7]
	if !ok {
		t.Fatal("route 7 should be registered")
	}
	if len(entity.options.Middlewares) != 3 {
		t.Fatalf("expect 3 middlewares, got %d", len(entity.options.Middlewares))
	}

	req := r.node.reqPool.Get().(*request)
	req.route = 7
	r.handle(req)

	expect := []string{"mw1", "mw2", "mw3", "handler"}
	if len(order) != len(expect) {
		t.Fatalf("expect %v, got %v", expect, order)
	}
	for i := range expect {
		if order[i] != expect[i] {
			t.Fatalf("expect %v, got %v", expect, order)
		}
	}

	// A group without explicit options still applies the group middlewares.
	group.AddRouteHandler(8, func(ctx Context) {})
	if entity, ok = r.routes[8]; !ok || len(entity.options.Middlewares) != 2 {
		t.Fatalf("route 8 should carry the group middlewares, got %+v", entity)
	}
}

// TestRouterDeliverAndReceive verifies writing a message to the routing queue.
func TestRouterDeliverAndReceive(t *testing.T) {
	t.Run("background context", func(t *testing.T) {
		n := newTestNode(t)
		r := n.router

		if err := r.deliver("gate-1", "node-2", "actor-1", 11, 22, 33, 44, []byte("hi")); err != nil {
			t.Fatalf("deliver failed: %v", err)
		}

		select {
		case req := <-r.receive():
			if req.gid != "gate-1" || req.nid != "node-2" || req.pid != "actor-1" {
				t.Fatalf("unexpected ids: %+v", req)
			}
			if req.cid != 11 || req.uid != 22 || req.seq != 33 || req.route != 44 {
				t.Fatalf("unexpected numbers: %+v", req)
			}
			req.release()
		case <-time.After(time.Second):
			t.Fatal("expect a request in the routing queue")
		}
	})

	t.Run("custom context", func(t *testing.T) {
		type ctxKey struct{}

		n := newTestNode(t, WithContextFunc(func() context.Context {
			return context.WithValue(context.Background(), ctxKey{}, "custom")
		}))
		r := n.router

		if err := r.deliver("", "", "", 0, 0, 0, 0, nil); err != nil {
			t.Fatalf("deliver failed: %v", err)
		}

		req := <-r.receive()
		if got := req.Context().Value(ctxKey{}); got != "custom" {
			t.Fatalf("expect custom context value, got %v", got)
		}
		req.release()
	})

	t.Run("closed queue", func(t *testing.T) {
		n := newTestNode(t)
		r := n.router

		r.close()

		if err := r.deliver("", "", "", 0, 0, 0, 0, nil); err == nil {
			t.Fatal("expect an error when delivering to a closed queue")
		}
	})
}

// TestRouterClean verifies that residual requests are released.
func TestRouterClean(t *testing.T) {
	n := newTestNode(t)
	r := n.router

	if err := r.deliver("", "", "", 0, 0, 0, 0, []byte("x")); err != nil {
		t.Fatalf("deliver failed: %v", err)
	}

	r.close()
	r.clean()
}

// TestRouterHandle verifies the route dispatching paths.
func TestRouterHandle(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		n := newTestNode(t)
		n.router.handle(nil)
	})

	t.Run("registered route", func(t *testing.T) {
		n := newTestNode(t)
		r := n.router

		var called atomic.Int32
		r.AddRouteHandler(1, func(ctx Context) { called.Add(1) })

		req := r.node.reqPool.Get().(*request)
		req.route = 1
		r.handle(req)

		if called.Load() != 1 {
			t.Fatalf("expect handler called once, got %d", called.Load())
		}
	})

	t.Run("default route", func(t *testing.T) {
		n := newTestNode(t)
		r := n.router

		var called atomic.Int32
		r.SetDefaultRouteHandler(func(ctx Context) { called.Add(1) })

		req := r.node.reqPool.Get().(*request)
		req.route = 100
		r.handle(req)

		if called.Load() != 1 {
			t.Fatalf("expect default handler called once, got %d", called.Load())
		}
	})

	t.Run("unregistered route without default", func(t *testing.T) {
		n := newTestNode(t)
		r := n.router

		req := r.node.reqPool.Get().(*request)
		req.route = 101
		r.handle(req)

		if req.route != 0 {
			t.Fatal("request should be recycled")
		}
	})

	t.Run("pre and post handlers", func(t *testing.T) {
		n := newTestNode(t)
		r := n.router

		var pre, post atomic.Int32
		r.SetPreRouteHandler(func(ctx Context) { pre.Add(1) })
		r.SetPostRouteHandler(func(ctx Context) { post.Add(1) })
		r.AddRouteHandler(1, func(ctx Context) {})

		req := r.node.reqPool.Get().(*request)
		req.route = 1
		r.handle(req)

		// The post handler runs asynchronously through the deferred recycle chain.
		deadline := time.Now().Add(time.Second)
		for post.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}

		if pre.Load() != 1 {
			t.Fatalf("expect pre handler called once, got %d", pre.Load())
		}
		if post.Load() != 1 {
			t.Fatalf("expect post handler called once, got %d", post.Load())
		}
	})
}

// TestMiddlewareSkipBoundary verifies the middleware index boundary.
func TestMiddlewareSkipBoundary(t *testing.T) {
	m := &Middleware{index: 3, middlewares: []MiddlewareHandler{
		func(m *Middleware, ctx Context) { t.Fatal("should not be called") },
	}}

	req := &request{ctx: context.Background()}
	m.Skip(req, 1)
}

// TestTriggerAddEventHandler verifies event handler registration.
func TestTriggerAddEventHandler(t *testing.T) {
	n := newTestNode(t)
	tr := n.trigger

	tr.addEventHandler(1, func(ctx Context) {})
	if _, ok := tr.events[1]; !ok {
		t.Fatal("event handler should be registered")
	}

	setWorking(n)
	tr.addEventHandler(2, func(ctx Context) {})
	if _, ok := tr.events[2]; ok {
		t.Fatal("event handler should not be registered while working")
	}
}

// TestTriggerTriggerAndReceive verifies writing an event to the event queue.
func TestTriggerTriggerAndReceive(t *testing.T) {
	t.Run("background context", func(t *testing.T) {
		n := newTestNode(t)
		tr := n.trigger

		if err := tr.trigger(5, "gate-1", 10, 20); err != nil {
			t.Fatalf("trigger failed: %v", err)
		}

		select {
		case evt := <-tr.receive():
			if evt.event != 5 || evt.gid != "gate-1" || evt.cid != 10 || evt.uid != 20 {
				t.Fatalf("unexpected event: %+v", evt)
			}
			evt.release()
		case <-time.After(time.Second):
			t.Fatal("expect an event in the queue")
		}
	})

	t.Run("custom context", func(t *testing.T) {
		type ctxKey struct{}

		n := newTestNode(t, WithContextFunc(func() context.Context {
			return context.WithValue(context.Background(), ctxKey{}, "custom")
		}))
		tr := n.trigger

		if err := tr.trigger(5, "", 0, 0); err != nil {
			t.Fatalf("trigger failed: %v", err)
		}

		evt := <-tr.receive()
		if got := evt.Context().Value(ctxKey{}); got != "custom" {
			t.Fatalf("expect custom context value, got %v", got)
		}
		evt.release()
	})

	t.Run("closed queue", func(t *testing.T) {
		n := newTestNode(t)
		tr := n.trigger

		tr.close()

		if err := tr.trigger(5, "", 0, 0); err == nil {
			t.Fatal("expect an error when triggering a closed queue")
		}
	})
}

// TestTriggerClean verifies that residual events are released.
func TestTriggerClean(t *testing.T) {
	n := newTestNode(t)
	tr := n.trigger

	if err := tr.trigger(5, "gate-1", 1, 2); err != nil {
		t.Fatalf("trigger failed: %v", err)
	}

	tr.close()
	tr.clean()
}

// TestTriggerHandle verifies the event handling paths.
func TestTriggerHandle(t *testing.T) {
	t.Run("nil event", func(t *testing.T) {
		n := newTestNode(t)
		n.trigger.handle(nil)
	})

	t.Run("registered event", func(t *testing.T) {
		n := newTestNode(t)
		tr := n.trigger

		var called atomic.Int32
		tr.addEventHandler(5, func(ctx Context) { called.Add(1) })

		evt := tr.node.evtPool.Get().(*event)
		evt.event = 5
		tr.handle(evt)

		if called.Load() != 1 {
			t.Fatalf("expect handler called once, got %d", called.Load())
		}
	})

	t.Run("unregistered event", func(t *testing.T) {
		n := newTestNode(t)
		tr := n.trigger

		evt := tr.node.evtPool.Get().(*event)
		evt.event = 6
		tr.handle(evt)

		if evt.event != 0 {
			t.Fatal("event should be recycled")
		}
	})
}

// TestQueueHelpersBackpressure ensures the router and trigger queues are constructed with the
// configured capacity.
func TestQueueHelpersBackpressure(t *testing.T) {
	n := newTestNode(t, WithMessageQueueSize(2), WithMessageWriteTimeout(50*time.Millisecond))

	if cap(n.router.receive()) != 2 {
		t.Fatalf("expect router queue capacity 2, got %d", cap(n.router.receive()))
	}
	if cap(n.trigger.receive()) != 2 {
		t.Fatalf("expect trigger queue capacity 2, got %d", cap(n.trigger.receive()))
	}
}
