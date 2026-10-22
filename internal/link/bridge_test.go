package link

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/internal/transporter/node"
	locatememory "github.com/dobyte/due/v2/locate/memory"
	"github.com/dobyte/due/v2/registry"
	registrymemory "github.com/dobyte/due/v2/registry/memory"
	"github.com/dobyte/due/v2/session"
)

func TestBridgeStatefulTargetAndIncarnation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	localProvider := &nodeProviderStub{deliver: make(chan nodeLinkDeliverRecord, 4)}
	local, err := node.NewServer(localProvider, &node.ServerOptions{Addr: "127.0.0.1:0", DisableRemote: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Start(); err != nil {
		t.Fatal(err)
	}
	defer local.Stop()
	remoteProvider := &nodeProviderStub{deliver: make(chan nodeLinkDeliverRecord, 4)}
	_, addr := startNodeServer(t, remoteProvider)
	store := locatememory.NewLocator()
	defer store.Close()
	if err := store.BindNode(ctx, 42, "node", "remote"); err != nil {
		t.Fatal(err)
	}
	opts := baseLinkOptions()
	opts.Locator = store
	l := NewNodeLinker(ctx, opts)
	localIns := &registry.ServiceInstance{ID: "local", Name: "node", Alias: "node", State: "work",
		Endpoint: local.Endpoint().String(), Metadata: local.Metadata(nil), Routes: []registry.Route{{ID: 2, Stateful: true}}}
	l.dispatcher.ReplaceServices(localIns, nodeService("remote", addr, registry.Route{ID: 2, Stateful: true}))
	if err := l.Deliver(ctx, &DeliverArgs{Route: 2, CID: 3, UID: 42, Buffer: []byte("remote")}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-remoteProvider.deliver:
		if string(got.data) != "remote" {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-localProvider.deliver:
		t.Fatal("local instance stole a remote binding")
	default:
	}
	if err := l.Deliver(ctx, &DeliverArgs{NID: "local", CID: 3, Buffer: []byte("local")}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-localProvider.deliver:
		if got.gid != opts.ID || string(got.data) != "local" {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	peer, err := l.doBuildClient("local")
	if err != nil {
		t.Fatal(err)
	}
	_ = local.Stop()
	if _, err := peer.GetState(ctx); !errors.Is(err, errors.ErrServerClosed) {
		t.Fatal("stale cached peer", err)
	}
	if _, err := l.doBuildClient("local"); !errors.Is(err, errors.ErrServerClosed) {
		t.Fatal("stale discovery should not dial TCP", err)
	}
}

func TestBridgeMixedGateFanout(t *testing.T) {
	for _, ack := range []bool{false, true} {
		t.Run(map[bool]string{true: "ack", false: "oneway"}[ack], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			collected := &collector{}
			capture := func(_ context.Context, _ session.Kind, _ bool, b buffer.Buffer) (int64, error) {
				collected.record(b)
				return 3, nil
			}
			local, err := gate.NewServer(&gateProviderStub{broadcastFn: capture}, &gate.ServerOptions{Addr: "127.0.0.1:0", DisableRemote: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := local.Start(); err != nil {
				t.Fatal(err)
			}
			defer local.Stop()
			_, addr := startGateServer(t, &gateProviderStub{broadcastFn: capture})
			l := NewGateLinker(ctx, baseLinkOptions())
			l.dispatcher.ReplaceServices(&registry.ServiceInstance{ID: "local", Endpoint: local.Endpoint().String(), Metadata: local.Metadata(nil)}, gateService("remote", addr))
			n, err := l.Broadcast(ctx, &BroadcastArgs{Kind: session.Conn, Ack: ack, Message: &Message{Route: 1, Data: []byte("fanout")}})
			if err != nil {
				t.Fatal(err)
			}
			if (ack && n != 6) || (!ack && n != 0) {
				t.Fatalf("count=%d", n)
			}
			assertIdenticalPayloads(t, collected.waitPayloads(t, 2))
		})
	}
}

func TestBridgeMemoryDiscoverySurvivesSetupCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := registrymemory.NewRegistry()
	defer r.Close()
	opts := baseLinkOptions()
	opts.Registry = r
	l := NewNodeLinker(ctx, opts)
	l.WatchClusterInstance()
	if err := r.Register(ctx, &registry.ServiceInstance{ID: "n", Name: cluster.Node.String(), Endpoint: "local://node"}); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, time.Second, func() bool { _, err := l.dispatcher.FindEndpoint("n"); return err == nil }) {
		t.Fatal("watcher stopped when setup context was cancelled")
	}
}

func TestBridgeGateOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p := &gateProviderStub{}
	s, err := gate.NewServer(p, &gate.ServerOptions{Addr: "127.0.0.1:0", DisableRemote: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	l := NewGateLinker(ctx, baseLinkOptions())
	l.dispatcher.ReplaceServices(&registry.ServiceInstance{ID: "g", Endpoint: s.Endpoint().String(), Metadata: s.Metadata(nil)})
	c, err := l.doBuildClient("g")
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		run  func() error
	}{
		{name: "bind", run: func() error { return c.Bind(ctx, 1, 2) }},
		{name: "unbind", run: func() error { return c.Unbind(ctx, 2) }},
		{name: "ip", run: func() error { _, err := c.GetIP(ctx, session.Conn, 1); return err }},
		{name: "online", run: func() error { _, err := c.IsOnline(ctx, session.Conn, 1); return err }},
		{name: "stat", run: func() error { _, err := c.Stat(ctx, session.Conn); return err }},
		{name: "disconnect", run: func() error { return c.Disconnect(ctx, session.Conn, 1, false) }},
		{name: "push", run: func() error { return c.Push(ctx, session.Conn, 1, false, buffer.NewBytes([]byte("x")), true) }},
		{name: "multicast", run: func() error {
			n, err := c.Multicast(ctx, session.Conn, []int64{1, 2}, false, buffer.NewBytes([]byte("x")), true)
			if n != 2 {
				t.Errorf("count=%d", n)
			}
			return err
		}},
		{name: "publish", run: func() error {
			n, err := c.Publish(ctx, "room", false, buffer.NewBytes([]byte("x")), true)
			if n != 2 {
				t.Errorf("count=%d", n)
			}
			return err
		}},
		{name: "subscribe", run: func() error { return c.Subscribe(ctx, session.Conn, []int64{1}, "room") }},
		{name: "unsubscribe", run: func() error { return c.Unsubscribe(ctx, session.Conn, []int64{1}, "room") }},
		{name: "state", run: func() error {
			state, err := c.GetState(ctx)
			if state != cluster.Work {
				t.Errorf("state=%v", state)
			}
			return err
		}},
		{name: "set-state", run: func() error { return c.SetState(ctx, cluster.Busy) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
