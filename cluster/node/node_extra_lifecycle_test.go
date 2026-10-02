package node

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

// TestNodeNewAndInit verifies the construction of a node and the Init hook.
func TestNodeNewAndInit(t *testing.T) {
	n := newTestNode(t)

	if n.Proxy() == nil {
		t.Fatal("proxy should not be nil")
	}
	if n.Proxy().Router() == nil {
		t.Fatal("router should not be nil")
	}
	if n.Proxy().Trigger() == nil {
		t.Fatal("trigger should not be nil")
	}
	if n.Name() != "test-node-name" {
		t.Fatalf("unexpected name: %s", n.Name())
	}
	if got := n.getState(); got != cluster.Shut {
		t.Fatalf("expect shut state, got %v", got)
	}
	if !n.isShut() {
		t.Fatal("a fresh node should be shut")
	}

	var initCount atomic.Int32
	n.Proxy().AddHookListener(cluster.Init, func(proxy *Proxy) {
		if proxy == nil {
			t.Error("proxy should not be nil in the init hook")
		}
		initCount.Add(1)
	})

	n.Init()

	if got := initCount.Load(); got != 1 {
		t.Fatalf("expect init hook called once, got %d", got)
	}
}

// TestNodeLifecycle verifies the full Start/Close/Destroy lifecycle, including the hook execution,
// the state transitions and the service instance registration.
func TestNodeLifecycle(t *testing.T) {
	transporter := &stubTransporter{}
	n, _, reg := stubNode(t, WithEncryptor(&stubEncryptor{}), WithTransporter(transporter))

	// Routes and events must be registered while the node is shut.
	n.Proxy().AddRouteHandler(1, func(ctx Context) {})
	n.Proxy().AddRouteHandler(2, func(ctx Context) {}, InternalRoute)
	n.Proxy().SetDefaultRouteHandler(func(ctx Context) {})
	n.Proxy().AddEventHandler(cluster.Connect, func(ctx Context) {})

	var (
		initCount    atomic.Int32
		startCount   atomic.Int32
		closeCount   atomic.Int32
		destroyCount atomic.Int32
	)

	n.Proxy().AddHookListener(cluster.Init, func(proxy *Proxy) { initCount.Add(1) })
	n.Proxy().AddHookListener(cluster.Start, func(proxy *Proxy) { startCount.Add(1) })
	n.Proxy().AddHookListener(cluster.Close, func(proxy *Proxy) { closeCount.Add(1) })
	n.Proxy().AddHookListener(cluster.Destroy, func(proxy *Proxy) { destroyCount.Add(1) })

	n.Init()
	n.Start()

	// Starting an already started node must be a no-op.
	n.Start()

	if got := n.getState(); got != cluster.Work {
		t.Fatalf("expect work state, got %v", got)
	}
	if n.isShut() {
		t.Fatal("node should not be shut after start")
	}
	if got := n.Proxy().GetState(); got != cluster.Work {
		t.Fatalf("proxy state mismatch: %v", got)
	}
	if got := n.Proxy().GetID(); got != "test-node-id" {
		t.Fatalf("unexpected proxy id: %s", got)
	}
	if got := n.Proxy().GetName(); got != "test-node-name" {
		t.Fatalf("unexpected proxy name: %s", got)
	}

	// The state may only be switched between Work and Busy on a running node.
	if err := n.setState(cluster.Busy); err != nil {
		t.Fatalf("set busy state failed: %v", err)
	}
	if err := n.setState(cluster.Busy); err != nil {
		t.Fatalf("idempotent set state failed: %v", err)
	}
	if err := n.setState(cluster.Hang); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}
	if got := n.Proxy().GetState(); got != cluster.Busy {
		t.Fatalf("expect busy state, got %v", got)
	}

	// Adding hooks/providers while the node works is rejected but must not panic.
	n.Proxy().AddHookListener(cluster.Init, func(proxy *Proxy) {})
	n.Proxy().AddServiceProvider("svc", "desc", struct{}{})

	n.Close()

	if got := n.getState(); got != cluster.Shut {
		t.Fatalf("expect shut state after close, got %v", got)
	}

	n.Destroy()
	n.Destroy() // idempotent

	if got := initCount.Load(); got != 1 {
		t.Fatalf("expect init hook once, got %d", got)
	}
	if got := startCount.Load(); got != 1 {
		t.Fatalf("expect start hook once, got %d", got)
	}
	if got := closeCount.Load(); got != 1 {
		t.Fatalf("expect close hook once, got %d", got)
	}
	if got := destroyCount.Load(); got != 1 {
		t.Fatalf("expect destroy hook once, got %d", got)
	}
	if got := reg.registerCount; got == 0 {
		t.Fatal("expect the node instance to be registered")
	}
	if got := reg.deregisterCount; got == 0 {
		t.Fatal("expect the node instance to be deregistered")
	}
}

// TestNodeLifecycleWithTransport verifies starting a node that exposes a transport service.
func TestNodeLifecycleWithTransport(t *testing.T) {
	transporter := &stubTransporter{server: &stubTransportServer{addr: "127.0.0.1:0"}}
	n, _, reg := stubNode(t, WithTransporter(transporter))

	n.Proxy().AddServiceProvider("echo", "desc", struct{}{})

	n.Start()

	if n.transporter == nil {
		t.Fatal("transport server should be started")
	}
	if transporter.server.registered != 1 {
		t.Fatalf("expect one registered service, got %d", transporter.server.registered)
	}
	if got := reg.registerCount; got != 2 {
		t.Fatalf("expect node and mesh instances registered, got %d", got)
	}
	if reg.lastRegistered == nil {
		t.Fatal("expect a registered instance")
	}

	n.Close()
	n.Destroy()

	if transporter.closeCount != 0 {
		// Close is only called by the component, not by the node.
		t.Fatalf("transporter close should not be called by the node, got %d", transporter.closeCount)
	}
}

// TestNodeCloseOnShutNode verifies that closing a shut node returns immediately.
func TestNodeCloseOnShutNode(t *testing.T) {
	n := newTestNode(t)

	done := make(chan struct{})
	go func() {
		n.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close on a shut node should not block")
	}
}

// TestNodeSetStateIllegal verifies the illegal state transitions.
func TestNodeSetStateIllegal(t *testing.T) {
	t.Run("state greater than busy", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		if err := n.setState(cluster.Hang); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("current state is shut", func(t *testing.T) {
		n := newTestNode(t)

		if err := n.setState(cluster.Work); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestNodeServiceInstances verifies the registration, refresh and deregistration of instances,
// including the error branches.
func TestNodeServiceInstances(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		n, _, reg := stubNode(t)
		n.instances = []*registry.ServiceInstance{{ID: "a"}, {ID: "b"}}

		if err := n.doRegisterServiceInstances(); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		if reg.registerCount != 2 {
			t.Fatalf("expect 2 registrations, got %d", reg.registerCount)
		}

		if err := n.doRefreshServiceInstances(cluster.Busy); err != nil {
			t.Fatalf("refresh failed: %v", err)
		}
		for _, ins := range n.instances {
			if ins.State != cluster.Busy.String() {
				t.Fatalf("expect busy state, got %s", ins.State)
			}
		}

		n.refreshServiceInstances()
	})

	t.Run("register error", func(t *testing.T) {
		n, _, reg := stubNode(t)
		reg.registerErr = errors.ErrServiceRegisterFailed
		n.instances = []*registry.ServiceInstance{{ID: "a"}}

		if err := n.doRegisterServiceInstances(); err == nil {
			t.Fatal("expect a register error")
		}

		// refreshServiceInstances must swallow the error instead of terminating the process.
		n.refreshServiceInstances(cluster.Work)
	})

	t.Run("deregister error", func(t *testing.T) {
		n, _, reg := stubNode(t)
		reg.deregisterErr = errors.ErrServiceDeregisterFailed
		n.instances = []*registry.ServiceInstance{{ID: "a"}}

		n.deregisterServiceInstances()
	})
}

// TestNodeTransportServer verifies the start and stop of the transport server.
func TestNodeTransportServer(t *testing.T) {
	t.Run("no transporter", func(t *testing.T) {
		n := newTestNode(t)

		n.startTransportServer()
		if n.transporter != nil {
			t.Fatal("transporter should be nil")
		}

		n.stopTransportServer()
	})

	t.Run("no service provider", func(t *testing.T) {
		transporter := &stubTransporter{}
		n := newTestNode(t, WithTransporter(transporter))

		n.startTransportServer()
		if n.transporter != nil {
			t.Fatal("transporter server should not be created without services")
		}
		if transporter.discovery == nil {
			t.Fatal("default discovery should be set")
		}
	})

	t.Run("with service provider", func(t *testing.T) {
		transporter := &stubTransporter{server: &stubTransportServer{addr: "127.0.0.1:0"}}
		n := newTestNode(t, WithTransporter(transporter))
		n.addServiceProvider("echo", "desc", struct{}{})

		n.startTransportServer()
		if n.transporter == nil {
			t.Fatal("transporter server should be created")
		}
		if transporter.server.registered != 1 {
			t.Fatalf("expect one registered service, got %d", transporter.server.registered)
		}

		n.stopTransportServer()
	})
}

// TestNodeWaitCounter verifies the wait counter helpers.
func TestNodeWaitCounter(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		var n *Node

		if n.doAddWait() {
			t.Fatal("nil node should not accept a wait")
		}
		if n.doDoneWait() {
			t.Fatal("nil node should not complete a wait")
		}
	})

	t.Run("shut node", func(t *testing.T) {
		n := newTestNode(t)

		if n.doAddWait() {
			t.Fatal("shut node should not accept a wait")
		}
		if n.doDoneWait() {
			t.Fatal("shut node should not complete a wait")
		}
	})

	t.Run("working node", func(t *testing.T) {
		n := newTestNode(t)
		setWorking(n)

		if !n.doAddWait() {
			t.Fatal("working node should accept a wait")
		}
		if !n.doDoneWait() {
			t.Fatal("working node should complete a wait")
		}
	})

	t.Run("hang node closes done", func(t *testing.T) {
		n := newTestNode(t)
		n.state.Store(int32(cluster.Hang))
		n.counter.Store(1)

		if !n.doDoneWait() {
			t.Fatal("hang node should complete the last wait")
		}

		select {
		case <-n.done:
		default:
			t.Fatal("done channel should be closed after the last wait")
		}
		if n.getState() != cluster.Shut {
			t.Fatalf("expect shut state, got %v", n.getState())
		}
	})
}

// TestNodeAddHookAndProviderWhileWorking verifies that hooks and providers are ignored while the
// node is running.
func TestNodeAddHookAndProviderWhileWorking(t *testing.T) {
	n := newTestNode(t)
	setWorking(n)

	n.addHookListener(cluster.Start, func(proxy *Proxy) {})
	if _, ok := n.hooks[cluster.Start]; ok {
		t.Fatal("hook should not be added while working")
	}

	n.addServiceProvider("echo", "desc", struct{}{})
	if len(n.services) != 0 {
		t.Fatal("service provider should not be added while working")
	}
}
