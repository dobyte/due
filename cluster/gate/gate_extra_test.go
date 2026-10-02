package gate

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
)

// gateAttr is a connection attribute implementation for tests.
type gateAttr struct {
	m map[any]any
}

func (a *gateAttr) Set(key, value any)      { a.m[key] = value }
func (a *gateAttr) Get(key any) (any, bool) { v, ok := a.m[key]; return v, ok }
func (a *gateAttr) Del(key any) bool        { _, ok := a.m[key]; delete(a.m, key); return ok }
func (a *gateAttr) Clear()                  { a.m = make(map[any]any) }
func (a *gateAttr) Visit(fn func(key, value any) bool) {
	for k, v := range a.m {
		if !fn(k, v) {
			return
		}
	}
}

// gateConn is a connection implementation for tests.
type gateConn struct {
	id     int64
	uid    int64
	attr   *gateAttr
	closed atomic.Bool
	pushed atomic.Int64
}

func newGateConn(id, uid int64) *gateConn {
	return &gateConn{id: id, uid: uid, attr: &gateAttr{m: make(map[any]any)}}
}

func (c *gateConn) ID() int64                     { return c.id }
func (c *gateConn) UID() int64                    { return c.uid }
func (c *gateConn) Attr() network.Attr            { return c.attr }
func (c *gateConn) Bind(uid int64) error          { c.uid = uid; return nil }
func (c *gateConn) Unbind() error                 { c.uid = 0; return nil }
func (c *gateConn) Push(buf buffer.Buffer) error  { c.pushed.Add(1); buf.Release(); return nil }
func (c *gateConn) State() network.ConnState      { return network.ConnOpened }
func (c *gateConn) Close(force ...bool) error     { c.closed.Store(true); return nil }
func (c *gateConn) LocalIP() (string, error)      { return "127.0.0.1", nil }
func (c *gateConn) LocalAddr() (net.Addr, error)  { return nil, nil }
func (c *gateConn) RemoteIP() (string, error)     { return "10.0.0.1", nil }
func (c *gateConn) RemoteAddr() (net.Addr, error) { return nil, nil }

// gateServer is a network server implementation for tests.
type gateServer struct {
	addr string

	started     atomic.Bool
	stopErr     error
	connect     network.ConnectHandler
	disconnect  network.DisconnectHandler
	receive     network.ReceiveHandler
	onStart     network.StartHandler
	onStop      network.CloseHandler
	onHeartbeat network.HeartbeatHandler
}

func (s *gateServer) Addr() string     { return s.addr }
func (s *gateServer) Start() error     { s.started.Store(true); return nil }
func (s *gateServer) Protocol() string { return "test" }
func (s *gateServer) Stop() error {
	s.started.Store(false)
	return s.stopErr
}
func (s *gateServer) OnStart(handler network.StartHandler)         { s.onStart = handler }
func (s *gateServer) OnStop(handler network.CloseHandler)          { s.onStop = handler }
func (s *gateServer) OnConnect(handler network.ConnectHandler)     { s.connect = handler }
func (s *gateServer) OnHeartbeat(handler network.HeartbeatHandler) { s.onHeartbeat = handler }
func (s *gateServer) OnReceive(handler network.ReceiveHandler)     { s.receive = handler }
func (s *gateServer) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnect = handler
}

// gateLocateWatcher is a locator watcher that stops the watch loop immediately.
type gateLocateWatcher struct{}

func (gateLocateWatcher) Next() ([]*locate.Event, error) { return nil, errors.ErrWatcherStopped }
func (gateLocateWatcher) Stop() error                    { return nil }

// gateRegistryWatcher is a registry watcher that stops the watch loop immediately.
type gateRegistryWatcher struct{}

func (gateRegistryWatcher) Next() ([]*registry.ServiceInstance, error) { return nil, context.Canceled }
func (gateRegistryWatcher) Stop() error                                { return nil }

// gateLocator is a locator implementation for tests.
type gateLocator struct {
	mu            sync.Mutex
	gates         map[int64]string
	nodes         map[int64]map[string]string
	bindErr       error
	unbindGateErr error
	locateGateErr error
	watchErr      error
}

func newGateLocator() *gateLocator {
	return &gateLocator{gates: make(map[int64]string), nodes: make(map[int64]map[string]string)}
}

func (l *gateLocator) Name() string { return "test-locator" }

func (l *gateLocator) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	if l.watchErr != nil {
		return nil, l.watchErr
	}
	return gateLocateWatcher{}, nil
}

func (l *gateLocator) BindGate(ctx context.Context, uid int64, gid string) error {
	if l.bindErr != nil {
		return l.bindErr
	}
	l.mu.Lock()
	l.gates[uid] = gid
	l.mu.Unlock()
	return nil
}

func (l *gateLocator) BindNode(ctx context.Context, uid int64, name, nid string) error {
	l.mu.Lock()
	if l.nodes[uid] == nil {
		l.nodes[uid] = make(map[string]string)
	}
	l.nodes[uid][name] = nid
	l.mu.Unlock()
	return nil
}

func (l *gateLocator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	if l.unbindGateErr != nil {
		return l.unbindGateErr
	}
	l.mu.Lock()
	delete(l.gates, uid)
	l.mu.Unlock()
	return nil
}

func (l *gateLocator) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	l.mu.Lock()
	delete(l.nodes[uid], name)
	l.mu.Unlock()
	return nil
}

func (l *gateLocator) LocateGate(ctx context.Context, uid int64) (string, error) {
	if l.locateGateErr != nil {
		return "", l.locateGateErr
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.gates[uid], nil
}

func (l *gateLocator) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.nodes[uid][name], nil
}

func (l *gateLocator) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.nodes[uid], nil
}

func (l *gateLocator) Close() error { return nil }

// gateRegistry is a registry implementation for tests.
type gateRegistry struct {
	mu           sync.Mutex
	registered   []*registry.ServiceInstance
	deregistered []*registry.ServiceInstance
	services     []*registry.ServiceInstance
	registerErr  error
	deregisterEr error
	watchErr     error
}

func (r *gateRegistry) Name() string { return "test-registry" }

func (r *gateRegistry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.registerErr != nil {
		return r.registerErr
	}
	r.mu.Lock()
	r.registered = append(r.registered, ins)
	r.mu.Unlock()
	return nil
}

func (r *gateRegistry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.deregisterEr != nil {
		return r.deregisterEr
	}
	r.mu.Lock()
	r.deregistered = append(r.deregistered, ins)
	r.mu.Unlock()
	return nil
}

func (r *gateRegistry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.watchErr != nil {
		return nil, r.watchErr
	}
	return gateRegistryWatcher{}, nil
}

func (r *gateRegistry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.services, nil
}

func (r *gateRegistry) Close() error { return nil }

// gateFixture bundles a gate and the test doubles it depends on.
type gateFixture struct {
	gate     *Gate
	server   *gateServer
	locator  *gateLocator
	registry *gateRegistry
}

func newGateFixture(t *testing.T) *gateFixture {
	t.Helper()

	f := &gateFixture{
		server:   &gateServer{addr: "127.0.0.1:0"},
		locator:  newGateLocator(),
		registry: &gateRegistry{},
	}

	f.gate = NewGate(
		WithID("test-gate-id"),
		WithName("test-gate-name"),
		WithServer(f.server),
		WithLocator(f.locator),
		WithRegistry(f.registry),
		WithAddr("127.0.0.1:0"),
	)

	return f
}

// start initializes and starts the gate, ensuring it is torn down when the test ends.
func (f *gateFixture) start(t *testing.T) {
	t.Helper()

	f.gate.Init()
	f.gate.Start()

	t.Cleanup(func() {
		f.gate.Close()
		f.gate.Destroy()
	})
}

func TestGateNewAndName(t *testing.T) {
	g := NewGate(WithName("my-gate"))

	if g.Name() != "my-gate" {
		t.Fatalf("unexpected name: %s", g.Name())
	}
	if g.getState() != cluster.Shut {
		t.Fatalf("a new gate must be shut, got %v", g.getState())
	}
	if !g.isShut() {
		t.Fatal("isShut must be true for a new gate")
	}
	if g.proxy == nil || g.session == nil || g.wg == nil {
		t.Fatal("gate internals must be initialized")
	}
}

func TestGateDefaultOptions(t *testing.T) {
	o := defaultOptions()

	if o.name != defaultName {
		t.Fatalf("unexpected default name: %s", o.name)
	}
	if o.addr != defaultAddr {
		t.Fatalf("unexpected default addr: %s", o.addr)
	}
	if o.dispatch != defaultDispatch {
		t.Fatalf("unexpected default dispatch: %v", o.dispatch)
	}
	if o.id == "" {
		t.Fatal("default id must not be empty")
	}
	if o.linker.connNum != defaultLinkerConnNum || o.linker.commandQueueSize != defaultLinkerCommandQueueSize {
		t.Fatalf("unexpected default linker options: %+v", o.linker)
	}
}

func TestGateOptions(t *testing.T) {
	tests := []struct {
		name  string
		apply func(o *options)
		check func(t *testing.T, o *options)
	}{
		{
			name:  "WithID valid",
			apply: func(o *options) { WithID("id-1")(o) },
			check: func(t *testing.T, o *options) {
				if o.id != "id-1" {
					t.Fatalf("unexpected id: %s", o.id)
				}
			},
		},
		{
			name:  "WithID empty",
			apply: func(o *options) { WithID("")(o) },
			check: func(t *testing.T, o *options) {
				if o.id == "" {
					t.Fatal("empty id must be ignored")
				}
			},
		},
		{
			name:  "WithName valid",
			apply: func(o *options) { WithName("n")(o) },
			check: func(t *testing.T, o *options) {
				if o.name != "n" {
					t.Fatalf("unexpected name: %s", o.name)
				}
			},
		},
		{
			name:  "WithName empty",
			apply: func(o *options) { WithName("")(o) },
			check: func(t *testing.T, o *options) {
				if o.name != defaultName {
					t.Fatalf("empty name must be ignored, got %s", o.name)
				}
			},
		},
		{
			name:  "WithContext valid",
			apply: func(o *options) { WithContext(context.Background())(o) },
			check: func(t *testing.T, o *options) {
				if o.ctx == nil {
					t.Fatal("ctx must be set")
				}
			},
		},
		{
			name:  "WithContext nil",
			apply: func(o *options) { WithContext(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.ctx == nil {
					t.Fatal("nil ctx must be ignored")
				}
			},
		},
		{
			name:  "WithServer valid",
			apply: func(o *options) { WithServer(&gateServer{})(o) },
			check: func(t *testing.T, o *options) {
				if o.server == nil {
					t.Fatal("server must be set")
				}
			},
		},
		{
			name:  "WithServer nil",
			apply: func(o *options) { WithServer(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.server != nil {
					t.Fatal("nil server must be ignored")
				}
			},
		},
		{
			name:  "WithLocator valid",
			apply: func(o *options) { WithLocator(newGateLocator())(o) },
			check: func(t *testing.T, o *options) {
				if o.locator == nil {
					t.Fatal("locator must be set")
				}
			},
		},
		{
			name:  "WithLocator nil",
			apply: func(o *options) { WithLocator(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.locator != nil {
					t.Fatal("nil locator must be ignored")
				}
			},
		},
		{
			name:  "WithRegistry valid",
			apply: func(o *options) { WithRegistry(&gateRegistry{})(o) },
			check: func(t *testing.T, o *options) {
				if o.registry == nil {
					t.Fatal("registry must be set")
				}
			},
		},
		{
			name:  "WithRegistry nil",
			apply: func(o *options) { WithRegistry(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.registry != nil {
					t.Fatal("nil registry must be ignored")
				}
			},
		},
		{
			name:  "WithDispatch valid",
			apply: func(o *options) { WithDispatch(cluster.RoundRobin)(o) },
			check: func(t *testing.T, o *options) {
				if o.dispatch != cluster.RoundRobin {
					t.Fatalf("unexpected dispatch: %v", o.dispatch)
				}
			},
		},
		{
			name:  "WithDispatch empty",
			apply: func(o *options) { WithDispatch("")(o) },
			check: func(t *testing.T, o *options) {
				if o.dispatch != defaultDispatch {
					t.Fatalf("empty dispatch must be ignored, got %v", o.dispatch)
				}
			},
		},
		{
			name:  "WithAddr valid",
			apply: func(o *options) { WithAddr("127.0.0.1:9000")(o) },
			check: func(t *testing.T, o *options) {
				if o.addr != "127.0.0.1:9000" {
					t.Fatalf("unexpected addr: %s", o.addr)
				}
			},
		},
		{
			name:  "WithAddr empty",
			apply: func(o *options) { WithAddr("")(o) },
			check: func(t *testing.T, o *options) {
				if o.addr != defaultAddr {
					t.Fatalf("empty addr must be ignored, got %s", o.addr)
				}
			},
		},
		{
			name:  "WithExpose",
			apply: func(o *options) { WithExpose(true)(o) },
			check: func(t *testing.T, o *options) {
				if !o.expose {
					t.Fatal("expose must be true")
				}
			},
		},
		{
			name:  "WithMetadata",
			apply: func(o *options) { WithMetadata(map[string]string{"k": "v"})(o) },
			check: func(t *testing.T, o *options) {
				if o.metadata["k"] != "v" {
					t.Fatalf("metadata not merged: %v", o.metadata)
				}
			},
		},
		{
			name:  "WithMetadata empty",
			apply: func(o *options) { WithMetadata(nil)(o) },
			check: func(t *testing.T, o *options) {
				if len(o.metadata) != 0 {
					t.Fatalf("empty metadata must be ignored: %v", o.metadata)
				}
			},
		},
		{
			name:  "WithLinkerConnNum valid",
			apply: func(o *options) { WithLinkerConnNum(8)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.connNum != 8 {
					t.Fatalf("unexpected connNum: %d", o.linker.connNum)
				}
			},
		},
		{
			name:  "WithLinkerConnNum invalid",
			apply: func(o *options) { WithLinkerConnNum(0)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.connNum != defaultLinkerConnNum {
					t.Fatalf("non-positive connNum must be ignored, got %d", o.linker.connNum)
				}
			},
		},
		{
			name:  "WithLinkerCallTimeout valid",
			apply: func(o *options) { WithLinkerCallTimeout(time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.callTimeout != time.Second {
					t.Fatalf("unexpected callTimeout: %v", o.linker.callTimeout)
				}
			},
		},
		{
			name:  "WithLinkerCallTimeout invalid",
			apply: func(o *options) { o.linker.callTimeout = 42 * time.Second; WithLinkerCallTimeout(-time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.callTimeout != 42*time.Second {
					t.Fatalf("negative callTimeout must be ignored, got %v", o.linker.callTimeout)
				}
			},
		},
		{
			name:  "WithLinkerDialTimeout valid",
			apply: func(o *options) { WithLinkerDialTimeout(time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.dialTimeout != time.Second {
					t.Fatalf("unexpected dialTimeout: %v", o.linker.dialTimeout)
				}
			},
		},
		{
			name:  "WithLinkerDialTimeout invalid",
			apply: func(o *options) { o.linker.dialTimeout = 42 * time.Second; WithLinkerDialTimeout(-time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.dialTimeout != 42*time.Second {
					t.Fatalf("negative dialTimeout must be ignored, got %v", o.linker.dialTimeout)
				}
			},
		},
		{
			name:  "WithLinkerDialRetryTimes valid",
			apply: func(o *options) { WithLinkerDialRetryTimes(2)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.dialRetryTimes != 2 {
					t.Fatalf("unexpected dialRetryTimes: %d", o.linker.dialRetryTimes)
				}
			},
		},
		{
			name:  "WithLinkerDialRetryTimes invalid",
			apply: func(o *options) { o.linker.dialRetryTimes = 42; WithLinkerDialRetryTimes(-1)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.dialRetryTimes != 42 {
					t.Fatalf("negative dialRetryTimes must be ignored, got %d", o.linker.dialRetryTimes)
				}
			},
		},
		{
			name:  "WithLinkerFaultRecoveryTime valid",
			apply: func(o *options) { WithLinkerFaultRecoveryTime(time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.faultRecoveryTime != time.Second {
					t.Fatalf("unexpected faultRecoveryTime: %v", o.linker.faultRecoveryTime)
				}
			},
		},
		{
			name: "WithLinkerFaultRecoveryTime invalid",
			apply: func(o *options) {
				o.linker.faultRecoveryTime = 42 * time.Second
				WithLinkerFaultRecoveryTime(-time.Second)(o)
			},
			check: func(t *testing.T, o *options) {
				if o.linker.faultRecoveryTime != 42*time.Second {
					t.Fatalf("negative faultRecoveryTime must be ignored, got %v", o.linker.faultRecoveryTime)
				}
			},
		},
		{
			name:  "WithLinkerCommandQueueSize valid",
			apply: func(o *options) { WithLinkerCommandQueueSize(16)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.commandQueueSize != 16 {
					t.Fatalf("unexpected commandQueueSize: %d", o.linker.commandQueueSize)
				}
			},
		},
		{
			name:  "WithLinkerCommandQueueSize invalid",
			apply: func(o *options) { WithLinkerCommandQueueSize(0)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.commandQueueSize != defaultLinkerCommandQueueSize {
					t.Fatalf("non-positive commandQueueSize must be ignored, got %d", o.linker.commandQueueSize)
				}
			},
		},
		{
			name:  "WithLinkerCommandWriteTimeout valid",
			apply: func(o *options) { WithLinkerCommandWriteTimeout(time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.commandWriteTimeout != time.Second {
					t.Fatalf("unexpected commandWriteTimeout: %v", o.linker.commandWriteTimeout)
				}
			},
		},
		{
			name:  "WithLinkerCommandWriteTimeout invalid",
			apply: func(o *options) { WithLinkerCommandWriteTimeout(-time.Second)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.commandWriteTimeout != 0 {
					t.Fatalf("negative commandWriteTimeout must be ignored, got %v", o.linker.commandWriteTimeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := defaultOptions()
			tt.apply(o)
			tt.check(t, o)
		})
	}
}

func TestGateLifecycle(t *testing.T) {
	f := newGateFixture(t)

	if f.gate.Name() != "test-gate-name" {
		t.Fatalf("unexpected name: %s", f.gate.Name())
	}

	f.gate.Init()
	f.gate.Start()

	if !f.server.started.Load() {
		t.Fatal("network server must be started")
	}
	if f.gate.getState() != cluster.Work {
		t.Fatalf("gate must be working, got %v", f.gate.getState())
	}
	if f.gate.instance == nil {
		t.Fatal("service instance must be registered")
	}
	if f.gate.linker == nil {
		t.Fatal("linker server must be started")
	}

	// Starting again must be a no-op.
	f.gate.Start()

	// The registered handlers must be wired to the gate.
	if f.server.connect == nil || f.server.disconnect == nil || f.server.receive == nil {
		t.Fatal("network handlers must be registered")
	}

	f.gate.Close()
	if f.gate.getState() != cluster.Hang {
		t.Fatalf("gate must be hanging after close, got %v", f.gate.getState())
	}

	// Closing again must be a no-op.
	f.gate.Close()

	f.gate.Destroy()
	if f.gate.getState() != cluster.Shut {
		t.Fatalf("gate must be shut after destroy, got %v", f.gate.getState())
	}
	if f.server.started.Load() {
		t.Fatal("network server must be stopped")
	}
	if len(f.registry.deregistered) == 0 {
		t.Fatal("service instance must be deregistered")
	}

	// Destroying again must be a no-op.
	f.gate.Destroy()
}

func TestGateCloseFromBusy(t *testing.T) {
	f := newGateFixture(t)
	f.gate.Init()
	f.gate.Start()
	t.Cleanup(f.gate.Destroy)

	f.gate.state.Store(int32(cluster.Busy))

	f.gate.Close()
	if f.gate.getState() != cluster.Hang {
		t.Fatalf("gate must be hanging, got %v", f.gate.getState())
	}
}

func TestGateStopNetworkServerError(t *testing.T) {
	f := newGateFixture(t)
	f.gate.Init()
	f.gate.Start()
	t.Cleanup(f.gate.Destroy)

	f.server.stopErr = errors.New("stop failed")
	f.gate.Close()

	if f.gate.getState() != cluster.Hang {
		t.Fatalf("gate must be hanging, got %v", f.gate.getState())
	}
}

func TestGateHandleConnectAndDisconnect(t *testing.T) {
	f := newGateFixture(t)
	f.start(t)

	conn := newGateConn(1001, 2001)
	f.server.connect(conn)

	if ok, err := f.gate.session.Has(session.Conn, conn.ID()); err != nil {
		t.Fatalf("session lookup failed: %v", err)
	} else if !ok {
		t.Fatal("conn must be registered after connect")
	}

	f.server.disconnect(conn)

	if ok, _ := f.gate.session.Has(session.Conn, conn.ID()); ok {
		t.Fatal("conn must be removed after disconnect")
	}
	if _, ok := f.locator.gates[2001]; ok {
		t.Fatal("user must be unbound from the locator")
	}

	// A duplicate disconnect must be ignored.
	f.server.disconnect(conn)
}

func TestGateHandleConnectRejected(t *testing.T) {
	f := newGateFixture(t)
	f.gate.Init()

	conn := newGateConn(1002, 0)
	f.gate.handleConnect(conn)

	if !conn.closed.Load() {
		t.Fatal("conn must be closed when the gate is shut")
	}
	if ok, _ := f.gate.session.Has(session.Conn, conn.ID()); ok {
		t.Fatal("conn must not be registered when the gate is shut")
	}
}

func TestGateHandleReceive(t *testing.T) {
	f := newGateFixture(t)
	f.start(t)

	conn := newGateConn(1003, 0)

	// A malformed buffer must be handled without panicking.
	f.gate.handleReceive(conn, buffer.NewBytes([]byte{1, 2, 3}, true))

	// A well-formed message without any route must be handled gracefully.
	buf, err := packet.PackMessage(&packet.Message{Route: 1, Seq: 1, Buffer: []byte("payload")})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.gate.handleReceive(conn, buf)
}

func TestGateSetState(t *testing.T) {
	f := newGateFixture(t)
	f.gate.instance = &registry.ServiceInstance{ID: "test"}

	// A shut gate rejects every state switch.
	if err := f.gate.setState(cluster.Work); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}
	if err := f.gate.setState(cluster.Hang); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}

	// A working gate may switch to busy and back.
	f.gate.state.Store(int32(cluster.Work))

	if err := f.gate.setState(cluster.Work); err != nil {
		t.Fatalf("switching to the same state must succeed: %v", err)
	}
	if err := f.gate.setState(cluster.Busy); err != nil {
		t.Fatalf("switching to busy must succeed: %v", err)
	}
	if f.gate.getState() != cluster.Busy {
		t.Fatalf("gate must be busy, got %v", f.gate.getState())
	}
	if err := f.gate.setState(cluster.Work); err != nil {
		t.Fatalf("switching back to work must succeed: %v", err)
	}
}

func TestGateDoRegisterServiceInstanceError(t *testing.T) {
	f := newGateFixture(t)
	f.gate.instance = &registry.ServiceInstance{ID: "test"}

	f.registry.registerErr = errors.New("register failed")
	if err := f.gate.doRegisterServiceInstance(); err == nil {
		t.Fatal("expect an error from doRegisterServiceInstance")
	}
}

func TestGateDeregisterServiceInstanceError(t *testing.T) {
	f := newGateFixture(t)
	f.gate.instance = &registry.ServiceInstance{ID: "test"}
	f.gate.ctx = context.Background()

	f.registry.deregisterEr = errors.New("deregister failed")
	f.gate.deregisterServiceInstance()
}

func TestGateRefreshServiceInstanceError(t *testing.T) {
	f := newGateFixture(t)
	f.gate.instance = &registry.ServiceInstance{ID: "test"}

	f.registry.registerErr = errors.New("refresh failed")
	f.gate.refreshServiceInstance()
}

func TestGateDoRefreshServiceInstance(t *testing.T) {
	f := newGateFixture(t)
	f.gate.instance = &registry.ServiceInstance{ID: "test"}

	if err := f.gate.doRefreshServiceInstance(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if err := f.gate.doRefreshServiceInstance(cluster.Busy); err != nil {
		t.Fatalf("refresh with state failed: %v", err)
	}
	if f.gate.instance.State != cluster.Busy.String() {
		t.Fatalf("instance state not refreshed: %s", f.gate.instance.State)
	}
}

func TestGateStopLinkerServerNil(t *testing.T) {
	g := NewGate()
	g.stopLinkerServer()
}

func TestGateProviderShutdown(t *testing.T) {
	g := NewGate(
		WithID("shut-gate"),
		WithServer(&gateServer{}),
		WithLocator(newGateLocator()),
		WithRegistry(&gateRegistry{}),
	)
	p := &provider{gate: g}

	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"Bind", func() error { return p.Bind(ctx, 1, 1) }},
		{"Unbind", func() error { return p.Unbind(ctx, 1) }},
		{"GetIP", func() error { _, err := p.GetIP(ctx, session.User, 1); return err }},
		{"IsOnline", func() error { _, err := p.IsOnline(ctx, session.User, 1); return err }},
		{"Stat", func() error { _, err := p.Stat(ctx, session.User); return err }},
		{"Disconnect", func() error { return p.Disconnect(ctx, session.User, 1, false) }},
		{"Push", func() error {
			return p.Push(ctx, session.User, 1, false, buffer.NewBytes(nil, true))
		}},
		{"Multicast", func() error {
			_, err := p.Multicast(ctx, session.User, []int64{1}, false, buffer.NewBytes(nil, true))
			return err
		}},
		{"Broadcast", func() error {
			_, err := p.Broadcast(ctx, session.User, false, buffer.NewBytes(nil, true))
			return err
		}},
		{"Publish", func() error {
			_, err := p.Publish(ctx, "ch", false, buffer.NewBytes(nil, true))
			return err
		}},
		{"Subscribe", func() error { return p.Subscribe(ctx, session.User, []int64{1}, "ch") }},
		{"Unsubscribe", func() error { return p.Unsubscribe(ctx, session.User, []int64{1}, "ch") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, errors.ErrGateShutdown) {
				t.Fatalf("expect ErrGateShutdown, got %v", err)
			}
		})
	}

	if st, err := p.GetState(); err != nil || st != cluster.Shut {
		t.Fatalf("unexpected state: %v, err: %v", st, err)
	}
}

func TestGateProviderBindUnbind(t *testing.T) {
	f := newGateFixture(t)
	f.start(t)

	p := &provider{gate: f.gate}
	ctx := context.Background()

	// Invalid arguments.
	if err := p.Bind(ctx, 0, 0); !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("expect ErrInvalidArgument, got %v", err)
	}
	if err := p.Unbind(ctx, 0); !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("expect ErrInvalidArgument, got %v", err)
	}

	// Binding an unknown connection fails.
	if err := p.Bind(ctx, 404, 1); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("expect ErrNotFoundSession, got %v", err)
	}

	conn := newGateConn(1, 0)
	f.gate.session.AddConn(conn)

	if err := p.Bind(ctx, conn.ID(), 100); err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	if got, _ := f.locator.LocateGate(ctx, 100); got != "test-gate-id" {
		t.Fatalf("user not bound to the gate, got: %s", got)
	}

	// Binding a user already bound to another connection twice is a no-op.
	if err := p.Bind(ctx, conn.ID(), 100); err != nil {
		t.Fatalf("rebinding the same user failed: %v", err)
	}

	if err := p.Unbind(ctx, 100); err != nil {
		t.Fatalf("unbind failed: %v", err)
	}
	if got, _ := f.locator.LocateGate(ctx, 100); got != "" {
		t.Fatalf("user not unbound from the gate, got: %s", got)
	}

	// Unbinding an unknown user fails.
	if err := p.Unbind(ctx, 999); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("expect ErrNotFoundSession, got %v", err)
	}
}

func TestGateProviderQueries(t *testing.T) {
	f := newGateFixture(t)
	f.start(t)

	p := &provider{gate: f.gate}
	ctx := context.Background()

	conn := newGateConn(7, 0)
	f.gate.session.AddConn(conn)

	ip, err := p.GetIP(ctx, session.Conn, conn.ID())
	if err != nil || ip != "10.0.0.1" {
		t.Fatalf("unexpected ip: %s, err: %v", ip, err)
	}

	online, err := p.IsOnline(ctx, session.Conn, conn.ID())
	if err != nil || !online {
		t.Fatalf("unexpected online: %v, err: %v", online, err)
	}

	total, err := p.Stat(ctx, session.Conn)
	if err != nil || total != 1 {
		t.Fatalf("unexpected stat: %d, err: %v", total, err)
	}

	if err = p.Disconnect(ctx, session.Conn, conn.ID(), false); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	if !conn.closed.Load() {
		t.Fatal("conn must be closed after disconnect")
	}

	// An invalid session kind propagates the session error.
	if _, err = p.GetIP(ctx, session.Kind(99), 1); !errors.Is(err, errors.ErrInvalidSessionKind) {
		t.Fatalf("expect ErrInvalidSessionKind, got %v", err)
	}
}

func TestGateProviderPushFamily(t *testing.T) {
	f := newGateFixture(t)
	f.start(t)

	p := &provider{gate: f.gate}
	ctx := context.Background()

	conn := newGateConn(11, 0)
	f.gate.session.AddConn(conn)

	if err := p.Push(ctx, session.Conn, conn.ID(), false, buffer.NewBytes(nil, true)); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if conn.pushed.Load() != 1 {
		t.Fatalf("conn must receive the push, got %d", conn.pushed.Load())
	}

	// Pushing to a missing user returns ErrNotFoundSession and schedules an unbind.
	f.locator.gates[500] = "test-gate-id"
	err := p.Push(ctx, session.User, 500, false, buffer.NewBytes(nil, true))
	if !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("expect ErrNotFoundSession, got %v", err)
	}

	total, err := p.Multicast(ctx, session.Conn, []int64{conn.ID()}, false, buffer.NewBytes(nil, true))
	if err != nil || total != 1 {
		t.Fatalf("unexpected multicast: %d, err: %v", total, err)
	}

	// An empty target list must be handled without pushing.
	total, err = p.Multicast(ctx, session.Conn, nil, false, buffer.NewBytes(nil, true))
	if err != nil || total != 0 {
		t.Fatalf("unexpected empty multicast: %d, err: %v", total, err)
	}

	total, err = p.Broadcast(ctx, session.Conn, false, buffer.NewBytes(nil, true))
	if err != nil || total != 1 {
		t.Fatalf("unexpected broadcast: %d, err: %v", total, err)
	}

	// No subscriber exists on the channel.
	total, err = p.Publish(ctx, "missing-channel", false, buffer.NewBytes(nil, true))
	if err != nil || total != 0 {
		t.Fatalf("unexpected publish: %d, err: %v", total, err)
	}
}

func TestGateProviderSubscribe(t *testing.T) {
	f := newGateFixture(t)
	f.start(t)

	p := &provider{gate: f.gate}
	ctx := context.Background()

	conn := newGateConn(21, 0)
	f.gate.session.AddConn(conn)

	if err := p.Subscribe(ctx, session.Conn, []int64{conn.ID()}, "chat"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	total, err := p.Publish(ctx, "chat", false, buffer.NewBytes(nil, true))
	if err != nil || total != 1 {
		t.Fatalf("unexpected publish to subscriber: %d, err: %v", total, err)
	}

	if err = p.Unsubscribe(ctx, session.Conn, []int64{conn.ID()}, "chat"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}

	// An invalid session kind propagates the session error.
	if err = p.Subscribe(ctx, session.Kind(99), []int64{1}, "chat"); !errors.Is(err, errors.ErrInvalidSessionKind) {
		t.Fatalf("expect ErrInvalidSessionKind, got %v", err)
	}
	if err = p.Unsubscribe(ctx, session.Kind(99), []int64{1}, "chat"); !errors.Is(err, errors.ErrInvalidSessionKind) {
		t.Fatalf("expect ErrInvalidSessionKind, got %v", err)
	}
}

func TestGateProviderSetState(t *testing.T) {
	f := newGateFixture(t)
	f.gate.instance = &registry.ServiceInstance{ID: "test"}

	p := &provider{gate: f.gate}

	f.gate.state.Store(int32(cluster.Work))

	if err := p.SetState(cluster.Busy); err != nil {
		t.Fatalf("set state failed: %v", err)
	}
	if st, err := p.GetState(); err != nil || st != cluster.Busy {
		t.Fatalf("unexpected state: %v, err: %v", st, err)
	}
	if err := p.SetState(cluster.Hang); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}
}

func TestGateProxyBindUnbind(t *testing.T) {
	f := newGateFixture(t)
	ctx := context.Background()

	// A shut gate rejects binding and unbinding.
	if err := f.gate.proxy.bindGate(ctx, 1, 1); !errors.Is(err, errors.ErrGateShutdown) {
		t.Fatalf("expect ErrGateShutdown, got %v", err)
	}
	if err := f.gate.proxy.unbindGate(ctx, 1, 1); !errors.Is(err, errors.ErrGateShutdown) {
		t.Fatalf("expect ErrGateShutdown, got %v", err)
	}

	f.gate.state.Store(int32(cluster.Work))

	if err := f.gate.proxy.bindGate(ctx, 1, 1); err != nil {
		t.Fatalf("bind gate failed: %v", err)
	}
	if got, _ := f.locator.LocateGate(ctx, 1); got != "test-gate-id" {
		t.Fatalf("user not bound, got: %s", got)
	}

	if err := f.gate.proxy.unbindGate(ctx, 1, 1); err != nil {
		t.Fatalf("unbind gate failed: %v", err)
	}

	// A locator failure is propagated.
	f.locator.unbindGateErr = errors.New("unbind failed")
	if err := f.gate.proxy.unbindGate(ctx, 1, 1); err == nil {
		t.Fatal("expect an error from unbindGate")
	}
}

func TestGateProxyBindGateLocatorError(t *testing.T) {
	f := newGateFixture(t)
	f.gate.state.Store(int32(cluster.Work))

	f.locator.bindErr = errors.New("bind failed")
	if err := f.gate.proxy.bindGate(context.Background(), 1, 1); err == nil {
		t.Fatal("expect an error from bindGate")
	}
}

func TestGateProxyTriggerAndDeliver(t *testing.T) {
	f := newGateFixture(t)
	ctx := context.Background()

	// A shut gate ignores triggers and releases delivered buffers.
	f.gate.proxy.trigger(ctx, cluster.Connect, 1, 1)
	f.gate.proxy.deliver(ctx, newGateConn(1, 1), buffer.NewBytes([]byte{1, 2, 3}, true))

	f.gate.state.Store(int32(cluster.Work))

	// No node is subscribed to the event.
	f.gate.proxy.trigger(ctx, cluster.Connect, 1, 1)

	// A malformed message is dropped.
	f.gate.proxy.deliver(ctx, newGateConn(1, 1), buffer.NewBytes([]byte{1, 2, 3}, true))

	// A well-formed message without a route is dropped.
	buf, err := packet.PackMessage(&packet.Message{Route: 1, Seq: 1, Buffer: []byte("payload")})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.gate.proxy.deliver(ctx, newGateConn(1, 1), buf)

	// The watch entry point must not panic.
	f.gate.proxy.watch()
}
