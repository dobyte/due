package gate

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	nodetransport "github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/registry"
)

// stoppedBeforeDisconnectServer models a transport skipping an already closing connection.
type stoppedBeforeDisconnectServer struct {
	*gateServer
	stopped chan struct{}
}

func (s *stoppedBeforeDisconnectServer) Stop() error {
	close(s.stopped)
	return nil
}

type disconnectRecord struct {
	cid, uid int64
}

// disconnectNode records teardown notifications received over the actual internal link.
type disconnectNode struct {
	events chan disconnectRecord
}

func (p *disconnectNode) Trigger(_ context.Context, _ string, cid, uid int64, event cluster.Event) error {
	if event == cluster.Disconnect {
		p.events <- disconnectRecord{cid: cid, uid: uid}
	}
	return nil
}

func (*disconnectNode) Deliver(_ context.Context, _, _ string, _, _ int64, buf buffer.Buffer) error {
	buf.Release()
	return nil
}

func (*disconnectNode) GetState() (cluster.State, error) { return cluster.Work, nil }
func (*disconnectNode) SetState(cluster.State) error     { return nil }

type disconnectRegistry struct {
	*gateRegistry
	instance *registry.ServiceInstance
}

func (r *disconnectRegistry) Watch(context.Context, string) (registry.Watcher, error) {
	return &disconnectWatcher{instance: r.instance}, nil
}

type disconnectWatcher struct {
	instance *registry.ServiceInstance
}

func (w *disconnectWatcher) Next() ([]*registry.ServiceInstance, error) {
	if w.instance == nil {
		return nil, errors.ErrWatcherStopped
	}
	ins := w.instance
	w.instance = nil
	return []*registry.ServiceInstance{ins}, nil
}

func (*disconnectWatcher) Stop() error { return nil }

func TestGateDestroyWaitsForDisconnectCleanup(t *testing.T) {
	recorder := &disconnectNode{events: make(chan disconnectRecord, 1)}
	server, err := nodetransport.NewServer(recorder, &nodetransport.ServerOptions{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	f := newGateFixture(t)
	g := f.gate
	t.Cleanup(g.cancel)
	g.opts.registry = &disconnectRegistry{gateRegistry: f.registry, instance: &registry.ServiceInstance{
		ID: "disconnect-node", Name: "node", Kind: "node", State: cluster.Work.String(),
		Endpoint: server.Endpoint().String(), Events: []int{int(cluster.Disconnect)},
	}}
	g.proxy = newProxy(g)
	g.proxy.nodeLinker.WatchClusterInstance()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for !g.proxy.nodeLinker.HasNode("disconnect-node") {
		select {
		case <-ctx.Done():
			t.Fatal("node was not discovered")
		case <-time.After(time.Millisecond):
		}
	}
	g.state.Store(int32(cluster.Work))
	conn := &securityConn{id: 1}
	g.handleConnect(conn)
	if err := g.session.Bind(1, 100); err != nil {
		t.Fatal(err)
	}
	if err := f.locator.BindGate(ctx, 100, g.opts.id); err != nil {
		t.Fatal(err)
	}
	network := &stoppedBeforeDisconnectServer{gateServer: f.server, stopped: make(chan struct{})}
	g.opts.server = network
	allowDisconnect := make(chan struct{})
	var release sync.Once
	done := make(chan struct{})
	t.Cleanup(func() {
		release.Do(func() { close(allowDisconnect) })
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Destroy did not finish after releasing disconnect")
		}
	})
	go func() {
		<-allowDisconnect
		g.handleDisconnect(conn)
	}()
	// Represents Close having entered Hang but exhausting its graceful shutdown budget.
	g.state.Store(int32(cluster.Hang))
	go func() { g.Destroy(); close(done) }()
	select {
	case <-network.stopped:
	case <-ctx.Done():
		t.Fatal("network did not stop")
	}
	if err := (&provider{gate: g}).Bind(ctx, 1, 200); !errors.Is(err, errors.ErrGateShutdown) {
		t.Fatalf("Destroy accepted a new binding: %v", err)
	}
	select {
	case <-done:
		t.Fatal("Destroy returned before the pending disconnect callback")
	case <-time.After(20 * time.Millisecond):
	}
	release.Do(func() { close(allowDisconnect) })
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Destroy did not wait for cleanup to finish")
	}
	if gid, _ := f.locator.LocateGate(ctx, 100); gid != "" {
		t.Fatalf("Destroy left a stale location: %q", gid)
	}
	select {
	case event := <-recorder.events:
		if event.cid != 1 || event.uid != 100 {
			t.Fatalf("unexpected disconnect: %+v", event)
		}
	case <-ctx.Done():
		t.Fatal("node did not receive Disconnect during Destroy")
	}
}
