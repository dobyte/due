package node

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/petermattis/goid"
)

// TestProxyShutPaths verifies that every guarded proxy call reports a shutdown error.
func TestProxyShutPaths(t *testing.T) {
	n := newTestNode(t)
	p := n.Proxy()
	ctx := context.Background()

	calls := []struct {
		name string
		fn   func() error
	}{
		{"BindGate", func() error { return p.BindGate(ctx, "gate-1", 1, 2) }},
		{"UnbindGate", func() error { return p.UnbindGate(ctx, 2) }},
		{"BindNode", func() error { return p.BindNode(ctx, 2) }},
		{"UnbindNode", func() error { return p.UnbindNode(ctx, 2) }},
		{"BindActor", func() error { return p.BindActor(2, "room", "1") }},
		{"UnbindActor", func() error { return p.UnbindActor(2, "room") }},
		{"Push", func() error { return p.Push(ctx, &cluster.PushArgs{}) }},
		{"Disconnect", func() error { return p.Disconnect(ctx, &cluster.DisconnectArgs{}) }},
		{"Subscribe", func() error { return p.Subscribe(ctx, &cluster.SubscribeArgs{}) }},
		{"Unsubscribe", func() error { return p.Unsubscribe(ctx, &cluster.UnsubscribeArgs{}) }},
		{"Deliver", func() error {
			return p.Deliver(ctx, &cluster.DeliverArgs{NID: "other", Message: &cluster.Message{}})
		}},
		{"Invoke", func() error { return p.Invoke(func() {}) }},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.fn(); !errors.Is(err, errors.ErrNodeShutdown) {
				t.Fatalf("expect ErrNodeShutdown, got %v", err)
			}
		})
	}

	t.Run("SetState", func(t *testing.T) {
		// SetState is not guarded by the shutdown state; a shut node reports an illegal
		// operation instead.
		if err := p.SetState(cluster.Work); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
	t.Run("AskGate", func(t *testing.T) {
		if _, _, err := p.AskGate(ctx, "gate-1", 1); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("LocateGate", func(t *testing.T) {
		if _, err := p.LocateGate(ctx, 1); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("AskNode", func(t *testing.T) {
		if _, _, err := p.AskNode(ctx, 1, "name", "nid"); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("LocateNode", func(t *testing.T) {
		if _, err := p.LocateNode(ctx, 1, "name"); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("LocateNodes", func(t *testing.T) {
		if _, err := p.LocateNodes(ctx, 1); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("FetchGateList", func(t *testing.T) {
		if _, err := p.FetchGateList(ctx); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("FetchNodeList", func(t *testing.T) {
		if _, err := p.FetchNodeList(ctx); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("GetIP", func(t *testing.T) {
		if _, err := p.GetIP(ctx, &cluster.GetIPArgs{}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("Stat", func(t *testing.T) {
		if _, err := p.Stat(ctx, session.Conn); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("IsOnline", func(t *testing.T) {
		if _, err := p.IsOnline(ctx, &cluster.IsOnlineArgs{}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("Multicast", func(t *testing.T) {
		if _, err := p.Multicast(ctx, &cluster.MulticastArgs{}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("Broadcast", func(t *testing.T) {
		if _, err := p.Broadcast(ctx, &cluster.BroadcastArgs{}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("Publish", func(t *testing.T) {
		if _, err := p.Publish(ctx, &cluster.PublishArgs{}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("HasGate", func(t *testing.T) {
		if _, err := p.HasGate("gate-1"); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("Spawn", func(t *testing.T) {
		if _, err := p.Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1")); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("Kill", func(t *testing.T) {
		if p.Kill("room", "1") {
			t.Fatal("expect kill to report false on a shut node")
		}
	})
	t.Run("NewMeshClient", func(t *testing.T) {
		if _, err := p.NewMeshClient("direct://127.0.0.1:1"); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("AfterFunc", func(t *testing.T) {
		if _, err := p.AfterFunc(time.Millisecond, func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
	t.Run("AfterInvoke", func(t *testing.T) {
		if _, err := p.AfterInvoke(time.Millisecond, func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
			t.Fatalf("expect ErrNodeShutdown, got %v", err)
		}
	})
}

// TestProxyUnguardedShutCalls verifies the proxy calls that are not guarded by the shutdown state.
func TestProxyUnguardedShutCalls(t *testing.T) {
	n := newTestNode(t)
	p := n.Proxy()

	if p.HasNode("nid") {
		t.Fatal("no node should be found")
	}
	if _, ok := p.Actor("room", "1"); ok {
		t.Fatal("no actor should be found")
	}
	if p.Router() == nil || p.Trigger() == nil {
		t.Fatal("router and trigger should not be nil")
	}
	if p.RouteGroup() == nil {
		t.Fatal("route group should not be nil")
	}
}

// TestProxyWorkingPaths verifies the proxy calls on a working node.
func TestProxyWorkingPaths(t *testing.T) {
	n, locator, _ := stubNode(t)
	locator.gid = "gate-1"
	locator.nid = "node-2"
	setWorking(n)

	p := n.Proxy()
	ctx := context.Background()

	if p.GetID() != "test-node-id" || p.GetName() != "test-node-name" {
		t.Fatalf("unexpected identity: %s/%s", p.GetID(), p.GetName())
	}
	if p.GetState() != cluster.Work {
		t.Fatalf("expect work state, got %v", p.GetState())
	}
	if err := p.SetState(cluster.Busy); err != nil {
		t.Fatalf("set state failed: %v", err)
	}

	// Configuration while working is rejected but must not panic.
	p.AddRouteHandler(1, func(ctx Context) {})
	p.SetDefaultRouteHandler(func(ctx Context) {})
	p.AddEventHandler(cluster.Connect, func(ctx Context) {})
	p.AddHookListener(cluster.Start, func(proxy *Proxy) {})
	p.AddServiceProvider("echo", "desc", struct{}{})

	if has, err := p.HasGate("gate-1"); err != nil || has {
		t.Fatalf("expect false and nil, got %v/%v", has, err)
	}

	gid, ok, err := p.AskGate(ctx, "gate-1", 100)
	if err != nil || !ok || gid != "gate-1" {
		t.Fatalf("unexpected ask gate result: %s/%v/%v", gid, ok, err)
	}

	if gid, err = p.LocateGate(ctx, 100); err != nil || gid != "gate-1" {
		t.Fatalf("unexpected locate gate result: %s/%v", gid, err)
	}

	if list, err := p.FetchGateList(ctx, cluster.Work); err != nil || len(list) != 0 {
		t.Fatalf("unexpected gate list: %v/%v", list, err)
	}

	if nid, ok, err := p.AskNode(ctx, 100, "name", "node-2"); err != nil || !ok || nid != "node-2" {
		t.Fatalf("unexpected ask node result: %s/%v/%v", nid, ok, err)
	}

	if nid, err := p.LocateNode(ctx, 100, "name"); err != nil || nid != "node-2" {
		t.Fatalf("unexpected locate node result: %s/%v", nid, err)
	}

	if nodes, err := p.LocateNodes(ctx, 100); err != nil || len(nodes) == 0 {
		t.Fatalf("unexpected locate nodes result: %v/%v", nodes, err)
	}

	if err := p.BindNode(ctx, 100); err != nil {
		t.Fatalf("bind node failed: %v", err)
	}
	if err := p.UnbindNode(ctx, 100); err != nil {
		t.Fatalf("unbind node failed: %v", err)
	}

	if list, err := p.FetchNodeList(ctx, cluster.Work); err != nil || len(list) != 0 {
		t.Fatalf("unexpected node list: %v/%v", list, err)
	}

	// Actor binding with a registered actor.
	registerTestActor(n.scheduler, "room", "1")
	if err := p.BindActor(100, "room", "1"); err != nil {
		t.Fatalf("bind actor failed: %v", err)
	}
	if err := p.UnbindActor(100, "room"); err != nil {
		t.Fatalf("unbind actor failed: %v", err)
	}
	if _, ok := p.Actor("room", "1"); !ok {
		t.Fatal("actor should exist")
	}
	if p.Kill("missing", "1") {
		t.Fatal("killing a missing actor should report false")
	}

	// Error paths of the gate and node calls (no endpoint is registered).
	if err := p.BindGate(ctx, "", 1, 2); err == nil {
		t.Fatal("expect an error for a missing gate id")
	}
	if _, err := p.GetIP(ctx, &cluster.GetIPArgs{Kind: session.User, Target: 100}); err == nil {
		t.Fatal("expect an error for a missing gate endpoint")
	}
	if _, err := p.Stat(ctx, session.Conn); err != nil {
		t.Fatalf("stat without endpoints should succeed, got %v", err)
	}
	if n, err := p.Broadcast(ctx, &cluster.BroadcastArgs{}); err != nil || n != 0 {
		t.Fatalf("broadcast without endpoints should return 0/nil, got %d/%v", n, err)
	}
	if n, err := p.Publish(ctx, &cluster.PublishArgs{}); err != nil || n != 0 {
		t.Fatalf("publish without endpoints should return 0/nil, got %d/%v", n, err)
	}
	if err := p.Deliver(ctx, &cluster.DeliverArgs{NID: "test-node-id"}); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation when delivering to self, got %v", err)
	}
}

// TestProxyPackMessage verifies packing helpers.
func TestProxyPackMessage(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)
	p := n.Proxy()

	t.Run("pack message", func(t *testing.T) {
		buf, err := p.PackMessage(&cluster.Message{Route: 1, Seq: 2, Data: []byte("x")})
		if err != nil {
			t.Fatalf("pack message failed: %v", err)
		}
		if len(buf) == 0 {
			t.Fatal("expect non-empty bytes")
		}
	})

	t.Run("pack message error", func(t *testing.T) {
		if _, err := p.PackMessage(&cluster.Message{Route: 1, Data: make(chan int)}); err == nil {
			t.Fatal("expect a marshal error")
		}
	})

	t.Run("pack buffer nil", func(t *testing.T) {
		buf, err := p.PackBuffer(nil)
		if err != nil || buf != nil {
			t.Fatalf("expect nil/nil, got %v/%v", buf, err)
		}
	})

	t.Run("pack buffer bytes", func(t *testing.T) {
		buf, err := p.PackBuffer([]byte("x"))
		if err != nil || string(buf) != "x" {
			t.Fatalf("unexpected pack buffer result: %v/%v", buf, err)
		}
	})

	t.Run("pack buffer with encryptor", func(t *testing.T) {
		en := newTestNode(t, WithEncryptor(&stubEncryptor{}))
		setWorking(en)

		buf, err := en.Proxy().PackBuffer(map[string]string{"a": "b"})
		if err != nil {
			t.Fatalf("pack buffer failed: %v", err)
		}
		if len(buf) == 0 {
			t.Fatal("expect non-empty bytes")
		}
	})
}

// TestProxyNewMeshClient verifies the mesh client helper.
func TestProxyNewMeshClient(t *testing.T) {
	t.Run("missing transporter", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		if _, err := n.Proxy().NewMeshClient("direct://127.0.0.1:1"); !errors.Is(err, errors.ErrMissingTransporter) {
			t.Fatalf("expect ErrMissingTransporter, got %v", err)
		}
	})

	t.Run("with transporter", func(t *testing.T) {
		n := newTestNode(t, WithTransporter(&stubTransporter{}))
		setWorking(n)

		if _, err := n.Proxy().NewMeshClient("direct://127.0.0.1:1"); err == nil {
			t.Fatal("expect an error from the stubbed transporter")
		}
	})
}

// TestProxyInvoke verifies the invoke helper of a proxy.
func TestProxyInvoke(t *testing.T) {
	t.Run("non blocking", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		if err := n.Proxy().Invoke(func() {}); err != nil {
			t.Fatalf("invoke failed: %v", err)
		}
	})

	t.Run("synchronous from dispatch goroutine", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		// Pretend the current goroutine is the dispatcher so that the synchronous path is taken.
		n.dispatchGoid.Store(goid.Get())

		var called atomic.Int32
		if err := n.Proxy().Invoke(func() { called.Store(1) }, true); err != nil {
			t.Fatalf("invoke failed: %v", err)
		}
		if called.Load() != 1 {
			t.Fatal("the function should run synchronously")
		}
	})
}

// TestProxyAfterFunc verifies the delayed call helpers of a proxy.
func TestProxyAfterFunc(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	done := make(chan struct{})
	timer, err := n.Proxy().AfterFunc(10*time.Millisecond, func() { close(done) })
	if err != nil {
		t.Fatalf("after func failed: %v", err)
	}
	if timer == nil {
		t.Fatal("timer should not be nil")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the after func should run")
	}

	if _, err = n.Proxy().AfterInvoke(time.Millisecond, func() {}); err != nil {
		t.Fatalf("after invoke failed: %v", err)
	}
}

// TestProxySpawnAndKill verifies the actor management helpers of a proxy.
func TestProxySpawnAndKill(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	if act == nil {
		t.Fatal("actor should not be nil")
	}
	if _, ok := n.Proxy().Actor("room", "1"); !ok {
		t.Fatal("actor should be registered")
	}
	if !n.Proxy().Kill("room", "1") {
		t.Fatal("expect the actor to be killed")
	}
	if n.Proxy().Kill("room", "1") {
		t.Fatal("the second kill should report false")
	}
}

// TestProxyDeliverOtherNode verifies delivering to another node.
func TestProxyDeliverOtherNode(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	err := n.Proxy().Deliver(context.Background(), &cluster.DeliverArgs{
		NID:     "other-node",
		UID:     100,
		Message: &cluster.Message{Route: 1, Data: []byte("x")},
	})
	if err == nil {
		t.Fatal("expect an error without a node endpoint")
	}
}

// TestProxyRouteGroup verifies the route group helper.
func TestProxyRouteGroup(t *testing.T) {
	n := newTestNode(t)

	group := n.Proxy().RouteGroup(func(g *RouterGroup) {
		g.Middleware(func(m *Middleware, ctx Context) { m.Next(ctx) })
	})
	if group == nil {
		t.Fatal("route group should not be nil")
	}

	group.AddRouteHandler(1, func(ctx Context) {})
	if _, ok := n.router.routes[1]; !ok {
		t.Fatal("route 1 should be registered")
	}
}

// TestProxySetDefaultRouteHandler verifies the default route handler helper.
func TestProxySetDefaultRouteHandler(t *testing.T) {
	n := newTestNode(t)

	n.Proxy().SetDefaultRouteHandler(func(ctx Context) {})
	if !n.router.HasDefaultRouteHandler() {
		t.Fatal("the default route handler should be set")
	}
}

// TestProxyGetStateOnBusy verifies the proxy state helpers with the registry check disabled.
func TestProxyGetStateOnBusy(t *testing.T) {
	n, _, reg := stubNode(t)
	setWorking(n)

	n.instances = []*registry.ServiceInstance{{ID: "a"}}

	// A failing registry must not break the state switch itself.
	reg.registerErr = errors.ErrServiceRegisterFailed

	if err := n.Proxy().SetState(cluster.Busy); err == nil {
		t.Fatal("expect the registry error to be reported")
	}
	if n.Proxy().GetState() != cluster.Busy {
		t.Fatalf("the state should be switched to busy, got %v", n.Proxy().GetState())
	}
}
