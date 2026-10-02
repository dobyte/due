package node

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
)

// newWorkingRequest creates a node in the work state together with a request context bound to it.
func newWorkingRequest(t *testing.T, opts ...Option) (*Node, *request) {
	t.Helper()

	n := newTestNode(t, opts...)
	setWorking(n)

	r := n.reqPool.Get().(*request)
	r.ctx = context.Background()

	return n, r
}

// TestRequestGetters verifies the request accessors.
func TestRequestGetters(t *testing.T) {
	r := &request{
		gid:   "gate-1",
		nid:   "node-2",
		pid:   "room/1",
		cid:   11,
		uid:   22,
		seq:   33,
		route: 44,
	}

	if r.GID() != "gate-1" || r.NID() != "node-2" || r.CID() != 11 || r.UID() != 22 {
		t.Fatalf("unexpected getters: %+v", r)
	}
	if r.Seq() != 33 || r.Route() != 44 {
		t.Fatalf("unexpected seq/route: %d/%d", r.Seq(), r.Route())
	}
	if r.Event() != 0 {
		t.Fatalf("request event should be zero, got %v", r.Event())
	}
	if r.Kind() != Request {
		t.Fatalf("expect request kind, got %v", r.Kind())
	}
}

// TestRequestParse verifies the message parsing paths.
func TestRequestParse(t *testing.T) {
	type payload struct {
		A int `json:"a"`
	}

	t.Run("bytes", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.message = []byte(`{"a":1}`)

		var v payload
		if err := r.Parse(&v); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if v.A != 1 {
			t.Fatalf("expect a=1, got %d", v.A)
		}
	})

	t.Run("empty bytes", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.message = []byte{}

		var v payload
		if err := r.Parse(&v); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
	})

	t.Run("buffer", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		buf := buffer.NewBytes([]byte(`{"a":2}`))
		r.message = buf

		var v payload
		if err := r.Parse(&v); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if v.A != 2 {
			t.Fatalf("expect a=2, got %d", v.A)
		}
	})

	t.Run("copier fallback", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.message = payload{A: 3}

		var v payload
		if err := r.Parse(&v); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if v.A != 3 {
			t.Fatalf("expect a=3, got %d", v.A)
		}
	})

	t.Run("encrypted", func(t *testing.T) {
		_, r := newWorkingRequest(t, WithEncryptor(&stubEncryptor{}))
		r.gid = "gate-1"
		r.message = []byte(`{"a":4}`)

		var v payload
		if err := r.Parse(&v); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if v.A != 4 {
			t.Fatalf("expect a=4, got %d", v.A)
		}
	})

	t.Run("decrypt error", func(t *testing.T) {
		_, r := newWorkingRequest(t, WithEncryptor(&stubEncryptor{decryptErr: errStub}))
		r.gid = "gate-1"
		r.message = []byte(`{"a":4}`)

		var v payload
		if err := r.Parse(&v); !errors.Is(err, errStub) {
			t.Fatalf("expect the decrypt error, got %v", err)
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		codec := newStubCodec()
		codec.unmarshalErr = errStub

		_, r := newWorkingRequest(t, WithCodec(codec))
		r.message = []byte(`{"a":4}`)

		var v payload
		if err := r.Parse(&v); !errors.Is(err, errStub) {
			t.Fatalf("expect the unmarshal error, got %v", err)
		}
	})
}

// errStub is a sentinel error used to verify error propagation.
var errStub = errors.New("stub error")

// TestRequestDeferAndCancel verifies the defer call chain.
func TestRequestDeferAndCancel(t *testing.T) {
	t.Run("head and tail", func(t *testing.T) {
		r := &request{}

		var order []int
		r.Defer(func() { order = append(order, 1) })
		r.Defer(func() { order = append(order, 2) })
		r.Defer(func() { order = append(order, 3) }, true)

		r.compareVersionExecDefer(r.loadVersion())

		if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 3 {
			t.Fatalf("unexpected defer order: %v", order)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		r := &request{}
		var called atomic.Int32
		r.Defer(func() { called.Add(1) })
		r.Cancel()
		r.compareVersionExecDefer(r.loadVersion())

		if called.Load() != 0 {
			t.Fatal("cancelled defer should not run")
		}
	})
}

// TestRequestCloneBasic verifies cloning a fully populated request.
func TestRequestCloneBasic(t *testing.T) {
	n, r := newWorkingRequest(t)
	r.gid = "gate-1"
	r.nid = "node-2"
	r.pid = "room/1"
	r.cid = 5
	r.uid = 6
	r.seq = 7
	r.route = 8
	r.message = []byte("hello")

	act := &Actor{}
	r.actor.Store(act)

	c := r.Clone().(*request)
	if c.gid != "gate-1" || c.nid != "node-2" || c.pid != "room/1" {
		t.Fatalf("unexpected clone ids: %+v", c)
	}
	if c.cid != 5 || c.uid != 6 || c.seq != 7 || c.route != 8 {
		t.Fatalf("unexpected clone numbers: %+v", c)
	}
	if c.actor.Load() != act {
		t.Fatal("the actor should be carried over to the clone")
	}
	if string(c.message.([]byte)) != "hello" {
		t.Fatalf("unexpected clone message: %v", c.message)
	}

	_ = n
}

// TestRequestTask verifies submitting an asynchronous task.
func TestRequestTask(t *testing.T) {
	t.Run("working node", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		done := make(chan struct{})
		r.Task(func(ctx Context) { close(done) })

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("task should be executed")
		}
	})

	t.Run("shut node", func(t *testing.T) {
		n := newTestNode(t)
		r := n.reqPool.Get().(*request)

		var called atomic.Int32
		r.Task(func(ctx Context) { called.Add(1) })

		time.Sleep(50 * time.Millisecond)
		if called.Load() != 0 {
			t.Fatal("task on a shut node should not run")
		}
	})
}

// TestRequestNext verifies the dispatch entry point error paths.
func TestRequestNext(t *testing.T) {
	t.Run("missing uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.Next(); !errors.Is(err, errors.ErrMissingDispatchStrategy) {
			t.Fatalf("expect ErrMissingDispatchStrategy, got %v", err)
		}
	})

	t.Run("unregistered route", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.uid = 1
		if err := r.Next(); !errors.Is(err, errors.ErrUnregisterRoute) {
			t.Fatalf("expect ErrUnregisterRoute, got %v", err)
		}
	})
}

// TestRequestProxyAndContext verifies the proxy and context helpers.
func TestRequestProxyAndContext(t *testing.T) {
	n, r := newWorkingRequest(t)

	if r.Proxy() != n.proxy {
		t.Fatal("proxy mismatch")
	}
	if r.Context() != r.ctx {
		t.Fatal("context mismatch")
	}

	type key struct{}
	r.SetValue(key{}, "v")
	if got := r.GetValue(key{}); got != "v" {
		t.Fatalf("expect value v, got %v", got)
	}
}

// TestRequestBindGate verifies the gateway binding paths.
func TestRequestBindGate(t *testing.T) {
	t.Run("explicit uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.gid = "gate-1"
		r.cid = 1

		// No gate endpoint is registered, so the binding is reported as an error.
		if err := r.BindGate(100); err == nil {
			t.Fatal("expect a binding error")
		}
		if r.uid != 0 {
			t.Fatal("uid should not be updated on failure")
		}
	})

	t.Run("current uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.uid = 100

		if err := r.BindGate(); err == nil {
			t.Fatal("expect a binding error")
		}
	})

	t.Run("missing uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if err := r.BindGate(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestRequestUnbindGate verifies the gateway unbinding paths.
func TestRequestUnbindGate(t *testing.T) {
	t.Run("explicit uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.UnbindGate(100); err == nil {
			t.Fatal("expect an unbinding error")
		}
	})

	t.Run("current uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.uid = 100
		if err := r.UnbindGate(); err == nil {
			t.Fatal("expect an unbinding error")
		}
	})

	t.Run("missing uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.UnbindGate(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestRequestBindNode verifies the node binding paths.
func TestRequestBindNode(t *testing.T) {
	t.Run("explicit uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.BindNode(100); err != nil {
			t.Fatalf("bind node failed: %v", err)
		}
	})

	t.Run("current uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.uid = 100
		if err := r.BindNode(); err != nil {
			t.Fatalf("bind node failed: %v", err)
		}
	})

	t.Run("missing uid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.BindNode(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("explicit uid unbind", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.UnbindNode(100); err != nil {
			t.Fatalf("unbind node failed: %v", err)
		}
	})

	t.Run("current uid unbind", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.uid = 100
		if err := r.UnbindNode(); err != nil {
			t.Fatalf("unbind node failed: %v", err)
		}
	})

	t.Run("missing uid unbind", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.UnbindNode(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestRequestSubscribe verifies the subscribe and unsubscribe paths.
func TestRequestSubscribe(t *testing.T) {
	t.Run("user targets", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.gid = "gate-1"

		// Locating the gate succeeds through the locator stub, but no endpoint is registered.
		_ = r.Subscribe("ch", 1, 2)
		_ = r.Unsubscribe("ch", 1, 2)
	})

	t.Run("conn target", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.gid = "gate-1"
		r.cid = 3

		_ = r.Subscribe("ch")
		_ = r.Unsubscribe("ch")
	})

	t.Run("missing gid", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if err := r.Subscribe("ch"); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
		if err := r.Unsubscribe("ch"); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestRequestActorHelpers verifies the actor helpers of a request.
func TestRequestActorHelpers(t *testing.T) {
	n, r := newWorkingRequest(t)
	registerTestActor(n.scheduler, "room", "1")
	r.uid = 100

	if err := r.BindActor("room", "1"); err != nil {
		t.Fatalf("bind actor failed: %v", err)
	}
	if _, ok := r.Actor("room", "1"); !ok {
		t.Fatal("actor should exist")
	}

	r.UnbindActor("room")

	if r.Kill("missing", "1") {
		t.Fatal("killing a missing actor should return false")
	}

	if _, err := r.Spawn(nil); !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("expect ErrInvalidArgument, got %v", err)
	}
}

// TestRequestInvoke verifies the invoke helper for both the global and the actor scope.
func TestRequestInvoke(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if err := r.Invoke(func() {}); err != nil {
			t.Fatalf("invoke failed: %v", err)
		}
	})

	t.Run("actor", func(t *testing.T) {
		n, r := newWorkingRequest(t)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("1"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		r.actor.Store(act)

		done := make(chan struct{})
		if err := r.Invoke(func() { close(done) }, true); err != nil {
			t.Fatalf("invoke failed: %v", err)
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("actor invoke should be executed")
		}
	})
}

// TestRequestAfterFunc verifies the delayed call helpers.
func TestRequestAfterFunc(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		done := make(chan struct{})
		timer, err := r.AfterFunc(10*time.Millisecond, func() { close(done) })
		if err != nil {
			t.Fatalf("after func failed: %v", err)
		}
		if timer == nil {
			t.Fatal("timer should not be nil")
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("after func should run")
		}
	})

	t.Run("global after invoke", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if _, err := r.AfterInvoke(time.Millisecond, func() {}); err != nil {
			t.Fatalf("after invoke failed: %v", err)
		}
	})

	t.Run("actor", func(t *testing.T) {
		n, r := newWorkingRequest(t)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("2"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		r.actor.Store(act)

		if _, err = r.AfterFunc(5*time.Millisecond, func() {}); err != nil {
			t.Fatalf("actor after func failed: %v", err)
		}
		if _, err = r.AfterInvoke(5*time.Millisecond, func() {}); err != nil {
			t.Fatalf("actor after invoke failed: %v", err)
		}

		// Let the delayed callbacks fire while the actor is still alive.
		time.Sleep(50 * time.Millisecond)
	})
}

// TestRequestGetIP verifies the client IP helper.
func TestRequestGetIP(t *testing.T) {
	t.Run("missing gid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if _, err := r.GetIP(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("with gid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.gid = "gate-1"

		if _, err := r.GetIP(); err == nil {
			t.Fatal("expect an error without a gate endpoint")
		}
	})
}

// TestRequestDeliver verifies the deliver helper.
func TestRequestDeliver(t *testing.T) {
	t.Run("self node", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if err := r.Deliver(&cluster.DeliverArgs{NID: "test-node-id"}); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("other node", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if err := r.Deliver(&cluster.DeliverArgs{NID: "other", Message: &cluster.Message{Route: 1}}); err == nil {
			t.Fatal("expect an error without a node endpoint")
		}
	})
}

// TestRequestReply verifies the reply paths.
func TestRequestReply(t *testing.T) {
	t.Run("from gateway", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.gid = "gate-1"

		if err := r.Reply(&cluster.Message{Route: 1}); err == nil {
			t.Fatal("expect an error without a gate endpoint")
		}
	})

	t.Run("from actor", func(t *testing.T) {
		n, r := newWorkingRequest(t)

		act, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("room"), WithActorID("3"))
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		t.Cleanup(func() { act.Destroy() })

		r.pid = act.PID()
		r.uid = 100

		if err = r.Reply(&cluster.Message{Route: 1}); err != nil {
			t.Fatalf("reply failed: %v", err)
		}
	})

	t.Run("from missing actor", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.pid = "room/404"

		if err := r.Reply(&cluster.Message{Route: 1}); err != nil {
			t.Fatalf("reply to a missing actor should be a no-op, got %v", err)
		}
	})

	t.Run("from self node", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.nid = "test-node-id"

		if err := r.Reply(&cluster.Message{Route: 1}); err != nil {
			t.Fatalf("reply from self node should be a no-op, got %v", err)
		}
	})

	t.Run("from other node", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.nid = "other-node"

		if err := r.Reply(&cluster.Message{Route: 1}); err == nil {
			t.Fatal("expect an error without a node endpoint")
		}
	})

	t.Run("illegal source", func(t *testing.T) {
		_, r := newWorkingRequest(t)

		if err := r.Reply(&cluster.Message{Route: 1}); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})
}

// TestRequestResponse verifies the response helper.
func TestRequestResponse(t *testing.T) {
	_, r := newWorkingRequest(t)
	r.gid = "gate-1"
	r.route = 1
	r.seq = 2

	if err := r.Response("payload"); err == nil {
		t.Fatal("expect an error without a gate endpoint")
	}
}

// TestRequestDisconnect verifies the disconnect helper.
func TestRequestDisconnect(t *testing.T) {
	t.Run("missing gid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if err := r.Disconnect(); !errors.Is(err, errors.ErrIllegalOperation) {
			t.Fatalf("expect ErrIllegalOperation, got %v", err)
		}
	})

	t.Run("with gid", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		r.gid = "gate-1"

		if err := r.Disconnect(true); err == nil {
			t.Fatal("expect an error without a gate endpoint")
		}
	})
}

// TestRequestNewMeshClient verifies the mesh client helper.
func TestRequestNewMeshClient(t *testing.T) {
	t.Run("missing transporter", func(t *testing.T) {
		_, r := newWorkingRequest(t)
		if _, err := r.NewMeshClient("direct://127.0.0.1:1"); !errors.Is(err, errors.ErrMissingTransporter) {
			t.Fatalf("expect ErrMissingTransporter, got %v", err)
		}
	})

	t.Run("with transporter", func(t *testing.T) {
		_, r := newWorkingRequest(t, WithTransporter(&stubTransporter{}))
		if _, err := r.NewMeshClient("direct://127.0.0.1:1"); err == nil {
			t.Fatal("expect an error from the stubbed transporter")
		}
	})
}

// TestRequestActorState verifies the actor state helpers.
func TestRequestActorState(t *testing.T) {
	r := &request{}
	act := &Actor{}

	if r.deleteActor(); r.actor.Load() != nil {
		t.Fatal("actor should be nil")
	}

	r.storeActor(act)
	if r.actor.Load() != act {
		t.Fatal("actor should be stored")
	}

	r.deleteActor()
	if r.actor.Load() != nil {
		t.Fatal("actor should be cleared")
	}
}

// TestRequestVersion verifies the version helpers.
func TestRequestVersion(t *testing.T) {
	r := &request{}

	if r.loadVersion() != 0 {
		t.Fatal("initial version should be zero")
	}
	if v := r.incrVersion(); v != 1 {
		t.Fatalf("expect version 1, got %d", v)
	}
	if v := r.decrVersion(); v != 0 {
		t.Fatalf("expect version 0, got %d", v)
	}
}

// TestRequestDeferHelpers verifies the defer helper methods.
func TestRequestDeferHelpers(t *testing.T) {
	r := &request{}
	r.Defer(func() {})

	r.cancelDefer()
	r.recoverDefer()
	r.releaseDefer()
}

// TestRequestCompareVersionRecycle verifies the version-based recycling.
func TestRequestCompareVersionRecycle(t *testing.T) {
	n := newTestNode(t)

	var post atomic.Int32
	n.router.SetPostRouteHandler(func(ctx Context) { post.Add(1) })

	setWorking(n)

	r := n.reqPool.Get().(*request)
	r.version.Store(5)
	r.compareVersionRecycle(5)

	if post.Load() != 1 {
		t.Fatalf("expect post handler called once, got %d", post.Load())
	}
	if r.route != 0 {
		t.Fatal("request should be recycled")
	}

	// A mismatched version must not recycle.
	r2 := n.reqPool.Get().(*request)
	r2.version.Store(7)
	r2.compareVersionRecycle(8)

	if r2.version.Load() != 7 {
		t.Fatal("a mismatched version should not recycle")
	}
}

// TestRequestRelease verifies releasing a request.
func TestRequestRelease(t *testing.T) {
	n := newTestNode(t)

	buf := buffer.NewBytes([]byte("hello"))
	r := &request{node: n, message: buf, ctx: context.Background(), chain: nil}
	r.Defer(func() {})
	r.release()

	if r.message != nil || r.cache != nil || r.chain != nil {
		t.Fatal("request should be fully reset")
	}
	if r.ctx != context.Background() {
		t.Fatal("context should be reset to background")
	}
}

// TestRequestInvokeOnShutNode verifies the invoke helper on a shut node.
func TestRequestInvokeOnShutNode(t *testing.T) {
	n := newTestNode(t)
	r := n.reqPool.Get().(*request)

	if err := r.Invoke(func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
		t.Fatalf("expect ErrNodeShutdown, got %v", err)
	}
}

// TestRequestAfterFuncOnShutNode verifies the delayed helpers on a shut node.
func TestRequestAfterFuncOnShutNode(t *testing.T) {
	n := newTestNode(t)
	r := n.reqPool.Get().(*request)

	if _, err := r.AfterFunc(time.Millisecond, func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
		t.Fatalf("expect ErrNodeShutdown, got %v", err)
	}
	if _, err := r.AfterInvoke(time.Millisecond, func() {}); !errors.Is(err, errors.ErrNodeShutdown) {
		t.Fatalf("expect ErrNodeShutdown, got %v", err)
	}
}

// TestRequestConcurrentClone verifies that concurrent clones are safe.
func TestRequestConcurrentClone(t *testing.T) {
	_, r := newWorkingRequest(t)
	r.message = []byte("hello")
	r.uid = 1

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := r.Clone().(*request)
			c.release()
		}()
	}
	wg.Wait()
}

// TestRequestUnbindActorAndKill verifies the actor cleanup helpers.
func TestRequestUnbindActorAndKill(t *testing.T) {
	n, r := newWorkingRequest(t)
	act := registerTestActor(n.scheduler, "room", "9")
	r.uid = 100

	r.UnbindActor("room")

	if _, ok := r.Actor("room", "9"); !ok {
		t.Fatal("actor should still be registered")
	}

	_, _ = n.scheduler.remove("room", "9")
	_ = act
}
