package mesh

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

// meshLocateWatcher is a locator watcher that stops the watch loop immediately.
type meshLocateWatcher struct{}

func (meshLocateWatcher) Next() ([]*locate.Event, error) { return nil, errors.ErrWatcherStopped }
func (meshLocateWatcher) Stop() error                    { return nil }

// meshRegistryWatcher is a registry watcher that stops the watch loop immediately.
type meshRegistryWatcher struct{}

func (meshRegistryWatcher) Next() ([]*registry.ServiceInstance, error) { return nil, context.Canceled }
func (meshRegistryWatcher) Stop() error                                { return nil }

// meshLocator is a locator implementation for tests.
type meshLocator struct {
	mu      sync.Mutex
	gates   map[int64]string
	nodes   map[int64]map[string]string
	watchEr error
}

func newMeshLocator() *meshLocator {
	return &meshLocator{gates: make(map[int64]string), nodes: make(map[int64]map[string]string)}
}

func (l *meshLocator) Name() string { return "test-locator" }

func (l *meshLocator) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	if l.watchEr != nil {
		return nil, l.watchEr
	}
	return meshLocateWatcher{}, nil
}

func (l *meshLocator) BindGate(ctx context.Context, uid int64, gid string) error {
	l.mu.Lock()
	l.gates[uid] = gid
	l.mu.Unlock()
	return nil
}

func (l *meshLocator) BindNode(ctx context.Context, uid int64, name, nid string) error {
	l.mu.Lock()
	if l.nodes[uid] == nil {
		l.nodes[uid] = make(map[string]string)
	}
	l.nodes[uid][name] = nid
	l.mu.Unlock()
	return nil
}

func (l *meshLocator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	l.mu.Lock()
	delete(l.gates, uid)
	l.mu.Unlock()
	return nil
}

func (l *meshLocator) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	l.mu.Lock()
	delete(l.nodes[uid], name)
	l.mu.Unlock()
	return nil
}

func (l *meshLocator) LocateGate(ctx context.Context, uid int64) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.gates[uid], nil
}

func (l *meshLocator) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.nodes[uid][name], nil
}

func (l *meshLocator) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.nodes[uid], nil
}

func (l *meshLocator) Close() error { return nil }

// meshRegistry is a registry implementation for tests.
type meshRegistry struct {
	mu           sync.Mutex
	services     []*registry.ServiceInstance
	watchEr      error
	registerEr   error
	deregisterEr error
}

func (r *meshRegistry) Name() string { return "test-registry" }

func (r *meshRegistry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	return r.registerEr
}

func (r *meshRegistry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	return r.deregisterEr
}

func (r *meshRegistry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.watchEr != nil {
		return nil, r.watchEr
	}
	return meshRegistryWatcher{}, nil
}

func (r *meshRegistry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.services, nil
}

func (r *meshRegistry) Close() error { return nil }

// meshTransportClient is a transport client implementation for tests.
type meshTransportClient struct{}

func (meshTransportClient) Call(ctx context.Context, service, method string, args any, reply any, opts ...any) error {
	return nil
}
func (meshTransportClient) Client() any { return nil }

// meshTransportServer is a transport server implementation for tests.
type meshTransportServer struct {
	started   atomic.Bool
	stopped   atomic.Bool
	services  sync.Map
	ep        *endpoint.Endpoint
	startEr   error
	stopEr    error
	registerE error
}

func (s *meshTransportServer) Start() error                 { s.started.Store(true); return s.startEr }
func (s *meshTransportServer) Stop() error                  { s.stopped.Store(true); return s.stopEr }
func (s *meshTransportServer) Addr() string                 { return "127.0.0.1:0" }
func (s *meshTransportServer) Scheme() string               { return "drpc" }
func (s *meshTransportServer) Endpoint() *endpoint.Endpoint { return s.ep }
func (s *meshTransportServer) RegisterService(desc, service any) error {
	if s.registerE != nil {
		return s.registerE
	}
	s.services.Store(desc, service)
	return nil
}

// meshTransporter is a transporter implementation for tests.
type meshTransporter struct {
	server       *meshTransportServer
	client       transport.Client
	newServerErr error
	newClientErr error
	discovery    registry.Discovery
	closed       atomic.Bool
}

func (t *meshTransporter) Name() string { return "test-transporter" }

func (t *meshTransporter) NewServer() (transport.Server, error) {
	if t.newServerErr != nil {
		return nil, t.newServerErr
	}
	return t.server, nil
}

func (t *meshTransporter) NewClient(target string) (transport.Client, error) {
	if t.newClientErr != nil {
		return nil, t.newClientErr
	}
	return t.client, nil
}

func (t *meshTransporter) SetDefaultDiscovery(discovery registry.Discovery) { t.discovery = discovery }

func (t *meshTransporter) Close() error { t.closed.Store(true); return nil }

// meshEncryptor is an encryptor implementation for tests.
type meshEncryptor struct{}

func (meshEncryptor) Name() string                        { return "test-encryptor" }
func (meshEncryptor) Encrypt(data []byte) ([]byte, error) { return data, nil }
func (meshEncryptor) Decrypt(data []byte) ([]byte, error) { return data, nil }

// meshFixture bundles a mesh and the test doubles it depends on.
type meshFixture struct {
	mesh        *Mesh
	locator     *meshLocator
	registry    *meshRegistry
	transporter *meshTransporter
	server      *meshTransportServer
}

func newMeshFixture(t *testing.T) *meshFixture {
	t.Helper()

	f := &meshFixture{
		locator:  newMeshLocator(),
		registry: &meshRegistry{},
		server:   &meshTransportServer{ep: endpoint.NewEndpoint("drpc", "127.0.0.1:0", false)},
	}
	f.transporter = &meshTransporter{server: f.server, client: meshTransportClient{}}

	f.mesh = NewMesh(
		WithID("test-mesh-id"),
		WithName("test-mesh-name"),
		WithCodec(json.DefaultCodec),
		WithLocator(f.locator),
		WithRegistry(f.registry),
		WithTransporter(f.transporter),
	)

	return f
}

func TestMeshNewAndName(t *testing.T) {
	m := NewMesh(WithName("my-mesh"))

	if m.Name() != "my-mesh" {
		t.Fatalf("unexpected name: %s", m.Name())
	}
	if m.getState() != cluster.Shut {
		t.Fatalf("a new mesh must be shut, got %v", m.getState())
	}
	if !m.isShut() {
		t.Fatal("isShut must be true for a new mesh")
	}
	if m.proxy == nil || m.hooks == nil {
		t.Fatal("mesh internals must be initialized")
	}
}

func TestMeshDefaultOptions(t *testing.T) {
	o := defaultOptions()

	if o.name != defaultName {
		t.Fatalf("unexpected default name: %s", o.name)
	}
	if o.codec == nil {
		t.Fatal("default codec must not be nil")
	}
	if o.weight != defaultWeight {
		t.Fatalf("unexpected default weight: %d", o.weight)
	}
	if o.id == "" {
		t.Fatal("default id must not be empty")
	}
	if o.linker.connNum != defaultLinkerConnNum || o.linker.commandQueueSize != defaultLinkerCommandQueueSize {
		t.Fatalf("unexpected default linker options: %+v", o.linker)
	}
}

func TestMeshOptions(t *testing.T) {
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
			name:  "WithCodec valid",
			apply: func(o *options) { WithCodec(json.DefaultCodec)(o) },
			check: func(t *testing.T, o *options) {
				if o.codec == nil {
					t.Fatal("codec must be set")
				}
			},
		},
		{
			name:  "WithCodec nil",
			apply: func(o *options) { o.codec = nil; WithCodec(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.codec != nil {
					t.Fatal("nil codec must be ignored")
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
			name:  "WithLocator valid",
			apply: func(o *options) { WithLocator(newMeshLocator())(o) },
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
			apply: func(o *options) { WithRegistry(&meshRegistry{})(o) },
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
			name:  "WithEncryptor valid",
			apply: func(o *options) { WithEncryptor(meshEncryptor{})(o) },
			check: func(t *testing.T, o *options) {
				if o.encryptor == nil {
					t.Fatal("encryptor must be set")
				}
			},
		},
		{
			name:  "WithEncryptor nil",
			apply: func(o *options) { WithEncryptor(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.encryptor != nil {
					t.Fatal("nil encryptor must be ignored")
				}
			},
		},
		{
			name:  "WithTransporter valid",
			apply: func(o *options) { WithTransporter(&meshTransporter{})(o) },
			check: func(t *testing.T, o *options) {
				if o.transporter == nil {
					t.Fatal("transporter must be set")
				}
			},
		},
		{
			name:  "WithTransporter nil",
			apply: func(o *options) { WithTransporter(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.transporter != nil {
					t.Fatal("nil transporter must be ignored")
				}
			},
		},
		{
			name:  "WithWeight valid",
			apply: func(o *options) { WithWeight(5)(o) },
			check: func(t *testing.T, o *options) {
				if o.weight != 5 {
					t.Fatalf("unexpected weight: %d", o.weight)
				}
			},
		},
		{
			name:  "WithWeight invalid",
			apply: func(o *options) { o.weight = 9; WithWeight(0)(o) },
			check: func(t *testing.T, o *options) {
				if o.weight != 9 {
					t.Fatalf("non-positive weight must be ignored, got %d", o.weight)
				}
			},
		},
		{
			name:  "WithMetadata valid",
			apply: func(o *options) { WithMetadata(map[string]string{"k": "v"})(o) },
			check: func(t *testing.T, o *options) {
				if o.metadata["k"] != "v" {
					t.Fatalf("metadata not merged: %v", o.metadata)
				}
			},
		},
		{
			name:  "WithMetadata empty",
			apply: func(o *options) { o.metadata = make(map[string]string); WithMetadata(nil)(o) },
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
			apply: func(o *options) { o.linker.connNum = 9; WithLinkerConnNum(0)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.connNum != 9 {
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
			apply: func(o *options) { o.linker.commandQueueSize = 9; WithLinkerCommandQueueSize(0)(o) },
			check: func(t *testing.T, o *options) {
				if o.linker.commandQueueSize != 9 {
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
			name: "WithLinkerCommandWriteTimeout invalid",
			apply: func(o *options) {
				o.linker.commandWriteTimeout = 42 * time.Second
				WithLinkerCommandWriteTimeout(-time.Second)(o)
			},
			check: func(t *testing.T, o *options) {
				if o.linker.commandWriteTimeout != 42*time.Second {
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

// waitFor waits until cond reports true, failing the test when it does not within the timeout.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("condition not met in time")
}

func TestMeshLifecycle(t *testing.T) {
	f := newMeshFixture(t)

	var inits, starts, closes, destroys atomic.Int32

	f.mesh.Proxy().AddServiceProvider("test-service", "desc", &struct{}{})
	f.mesh.Proxy().AddHookListener(cluster.Init, func(p *Proxy) { inits.Add(1) })
	f.mesh.Proxy().AddHookListener(cluster.Start, func(p *Proxy) { starts.Add(1) })
	f.mesh.Proxy().AddHookListener(cluster.Close, func(p *Proxy) { closes.Add(1) })
	f.mesh.Proxy().AddHookListener(cluster.Destroy, func(p *Proxy) { destroys.Add(1) })

	f.mesh.Init()
	f.mesh.Start()

	t.Cleanup(func() {
		f.mesh.Close()
		f.mesh.Destroy()
	})

	if inits.Load() != 1 || starts.Load() != 1 {
		t.Fatalf("unexpected hook counts: init=%d start=%d", inits.Load(), starts.Load())
	}
	if !f.server.started.Load() {
		waitFor(t, f.server.started.Load)
	}
	if !f.server.started.Load() {
		t.Fatal("transport server must be started")
	}
	if f.transporter.discovery == nil {
		t.Fatal("default discovery must be set")
	}
	if f.mesh.instance == nil {
		t.Fatal("service instance must be registered")
	}
	if f.mesh.instance.Endpoint != f.server.ep.String() {
		t.Fatalf("unexpected instance endpoint: %s", f.mesh.instance.Endpoint)
	}

	// Starting again must be a no-op.
	f.mesh.Start()

	f.mesh.Close()
	if f.mesh.getState() != cluster.Hang {
		t.Fatalf("mesh must be hanging, got %v", f.mesh.getState())
	}
	if closes.Load() != 1 {
		t.Fatalf("unexpected close count: %d", closes.Load())
	}

	f.mesh.Destroy()
	if f.mesh.getState() != cluster.Shut {
		t.Fatalf("mesh must be shut, got %v", f.mesh.getState())
	}
	if !f.server.stopped.Load() {
		t.Fatal("transport server must be stopped")
	}
	if destroys.Load() != 1 {
		t.Fatalf("unexpected destroy count: %d", destroys.Load())
	}
}

func TestMeshProxyReturned(t *testing.T) {
	f := newMeshFixture(t)

	if f.mesh.Proxy() == nil {
		t.Fatal("proxy must not be nil")
	}
	if f.mesh.Proxy() != f.mesh.proxy {
		t.Fatal("proxy must be the same instance")
	}
}

func TestMeshAddHooksAndServicesWhileWorking(t *testing.T) {
	f := newMeshFixture(t)
	f.mesh.Proxy().AddServiceProvider("svc", "desc", &struct{}{})
	f.mesh.Init()
	f.mesh.Start()
	t.Cleanup(func() {
		f.mesh.Close()
		f.mesh.Destroy()
	})

	// Adding listeners and providers while working must be ignored without panicking.
	f.mesh.Proxy().AddHookListener(cluster.Start, func(p *Proxy) {})
	f.mesh.Proxy().AddServiceProvider("late", "desc", &struct{}{})

	if len(f.mesh.services) != 1 {
		t.Fatalf("late service provider must be ignored, got %d", len(f.mesh.services))
	}
}

func TestMeshSetState(t *testing.T) {
	f := newMeshFixture(t)
	f.mesh.instance = &registry.ServiceInstance{ID: "test"}

	// A shut mesh rejects every state switch.
	if err := f.mesh.setState(cluster.Work); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}

	f.mesh.state.Store(int32(cluster.Work))

	if err := f.mesh.setState(cluster.Work); err != nil {
		t.Fatalf("switching to the same state must succeed: %v", err)
	}
	if err := f.mesh.setState(cluster.Busy); err != nil {
		t.Fatalf("switching to busy must succeed: %v", err)
	}
	if f.mesh.getState() != cluster.Busy {
		t.Fatalf("mesh must be busy, got %v", f.mesh.getState())
	}
	if err := f.mesh.setState(cluster.Hang); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}
}

func TestMeshStopTransportServerEdges(t *testing.T) {
	// A mesh without a transport server ignores the stop call.
	NewMesh().stopTransportServer()

	f := newMeshFixture(t)
	f.mesh.transporter = f.server
	f.server.stopEr = errors.New("stop failed")
	f.mesh.stopTransportServer()
}

func TestMeshRegistryErrorPaths(t *testing.T) {
	f := newMeshFixture(t)
	f.mesh.instance = &registry.ServiceInstance{ID: "test"}

	f.registry.registerEr = errors.New("refresh failed")
	f.mesh.refreshServiceInstance()

	f.registry.deregisterEr = errors.New("deregister failed")
	f.mesh.deregisterServiceInstance()
}

func TestMeshProxyShutdown(t *testing.T) {
	f := newMeshFixture(t)
	p := f.mesh.Proxy()
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"NewMeshClient", func() error { _, err := p.NewMeshClient("direct://127.0.0.1:1"); return err }},
		{"HasGate", func() error { _, err := p.HasGate("gid"); return err }},
		{"AskGate", func() error { _, _, err := p.AskGate(ctx, "gid", 1); return err }},
		{"LocateGate", func() error { _, err := p.LocateGate(ctx, 1); return err }},
		{"BindGate", func() error { return p.BindGate(ctx, "gid", 1, 1) }},
		{"UnbindGate", func() error { return p.UnbindGate(ctx, 1) }},
		{"FetchGateList", func() error { _, err := p.FetchGateList(ctx); return err }},
		{"HasNode", func() error { _, err := p.HasNode("nid"); return err }},
		{"AskNode", func() error { _, _, err := p.AskNode(ctx, 1, "name", "nid"); return err }},
		{"LocateNode", func() error { _, err := p.LocateNode(ctx, 1, "name"); return err }},
		{"LocateNodes", func() error { _, err := p.LocateNodes(ctx, 1); return err }},
		{"BindNode", func() error { return p.BindNode(ctx, 1, "name", "nid") }},
		{"UnbindNode", func() error { return p.UnbindNode(ctx, 1, "name", "nid") }},
		{"FetchNodeList", func() error { _, err := p.FetchNodeList(ctx); return err }},
		{"GetIP", func() error { _, err := p.GetIP(ctx, &cluster.GetIPArgs{}); return err }},
		{"Stat", func() error { _, err := p.Stat(ctx, session.User); return err }},
		{"IsOnline", func() error { _, err := p.IsOnline(ctx, &cluster.IsOnlineArgs{}); return err }},
		{"Disconnect", func() error { return p.Disconnect(ctx, &cluster.DisconnectArgs{}) }},
		{"Push", func() error { return p.Push(ctx, &cluster.PushArgs{}) }},
		{"Multicast", func() error { _, err := p.Multicast(ctx, &cluster.MulticastArgs{}); return err }},
		{"Broadcast", func() error { _, err := p.Broadcast(ctx, &cluster.BroadcastArgs{}); return err }},
		{"Publish", func() error { _, err := p.Publish(ctx, &cluster.PublishArgs{}); return err }},
		{"Subscribe", func() error { return p.Subscribe(ctx, &cluster.SubscribeArgs{}) }},
		{"Unsubscribe", func() error { return p.Unsubscribe(ctx, &cluster.UnsubscribeArgs{}) }},
		{"Deliver", func() error { return p.Deliver(ctx, &cluster.DeliverArgs{Message: &cluster.Message{}}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, errors.ErrMeshShutdown) {
				t.Fatalf("expect ErrMeshShutdown, got %v", err)
			}
		})
	}
}

func TestMeshProxyWorking(t *testing.T) {
	f := newMeshFixture(t)
	f.mesh.state.Store(int32(cluster.Work))
	p := f.mesh.Proxy()
	ctx := context.Background()

	if p.GetID() != "test-mesh-id" {
		t.Fatalf("unexpected id: %s", p.GetID())
	}
	if p.GetName() != "test-mesh-name" {
		t.Fatalf("unexpected name: %s", p.GetName())
	}
	if p.GetState() != cluster.Work {
		t.Fatalf("unexpected state: %v", p.GetState())
	}

	// The mesh client factory delegates to the transporter.
	if _, err := p.NewMeshClient("direct://127.0.0.1:1"); err != nil {
		t.Fatalf("new mesh client failed: %v", err)
	}

	// Empty dispatchers produce negative results without error.
	if ok, err := p.HasGate("gid"); err != nil || ok {
		t.Fatalf("unexpected HasGate: %v, err: %v", ok, err)
	}
	if ok, err := p.HasNode("nid"); err != nil || ok {
		t.Fatalf("unexpected HasNode: %v, err: %v", ok, err)
	}

	// Locating an unknown user fails with ErrNotFoundUserLocation.
	if _, _, err := p.AskGate(ctx, "gid", 1); !errors.Is(err, errors.ErrNotFoundUserLocation) {
		t.Fatalf("expect ErrNotFoundUserLocation, got %v", err)
	}
	if _, err := p.LocateGate(ctx, 1); !errors.Is(err, errors.ErrNotFoundUserLocation) {
		t.Fatalf("expect ErrNotFoundUserLocation, got %v", err)
	}

	// Binding a gate without a registered endpoint fails.
	if err := p.BindGate(ctx, "", 1, 1); !errors.Is(err, errors.ErrInvalidGID) {
		t.Fatalf("expect ErrInvalidGID, got %v", err)
	}

	// Unbinding an unknown user fails while locating its gate.
	if err := p.UnbindGate(ctx, 1); !errors.Is(err, errors.ErrNotFoundUserLocation) {
		t.Fatalf("expect ErrNotFoundUserLocation, got %v", err)
	}

	// Node location uses the locator.
	f.locator.BindNode(ctx, 100, "room", "node-1")
	if nid, err := p.LocateNode(ctx, 100, "room"); err != nil || nid != "node-1" {
		t.Fatalf("unexpected LocateNode: %s, err: %v", nid, err)
	}
	if nodes, err := p.LocateNodes(ctx, 100); err != nil || nodes["room"] != "node-1" {
		t.Fatalf("unexpected LocateNodes: %v, err: %v", nodes, err)
	}
	if _, _, err := p.AskNode(ctx, 100, "room", "node-1"); err != nil {
		t.Fatalf("AskNode failed: %v", err)
	}
	if err := p.BindNode(ctx, 200, "room", "node-2"); err != nil {
		t.Fatalf("BindNode failed: %v", err)
	}
	if err := p.UnbindNode(ctx, 200, "room", "node-2"); err != nil {
		t.Fatalf("UnbindNode failed: %v", err)
	}

	// Service lists come from the registry.
	f.registry.services = []*registry.ServiceInstance{{ID: "g1", State: cluster.Work.String()}}
	if gates, err := p.FetchGateList(ctx); err != nil || len(gates) != 1 {
		t.Fatalf("unexpected FetchGateList: %v, err: %v", gates, err)
	}
	if gates, err := p.FetchGateList(ctx, cluster.Work); err != nil || len(gates) != 1 {
		t.Fatalf("unexpected filtered FetchGateList: %v, err: %v", gates, err)
	}
	if nodes, err := p.FetchNodeList(ctx); err != nil || len(nodes) != 1 {
		t.Fatalf("unexpected FetchNodeList: %v, err: %v", nodes, err)
	}
	if nodes, err := p.FetchNodeList(ctx, cluster.Work); err != nil || len(nodes) != 1 {
		t.Fatalf("unexpected filtered FetchNodeList: %v, err: %v", nodes, err)
	}

	// Message packing delegates to the gate linker.
	if buf, err := p.PackMessage(&cluster.Message{Route: 1, Data: map[string]string{"k": "v"}}); err != nil || len(buf) == 0 {
		t.Fatalf("unexpected PackMessage: %d, err: %v", len(buf), err)
	}
	if buf, err := p.PackBuffer(map[string]string{"k": "v"}); err != nil || len(buf) == 0 {
		t.Fatalf("unexpected PackBuffer: %d, err: %v", len(buf), err)
	}
	if buf, err := p.PackBuffer(nil); err != nil || buf != nil {
		t.Fatalf("unexpected PackBuffer(nil): %v, err: %v", buf, err)
	}

	// Stat with an empty dispatcher returns zero.
	if total, err := p.Stat(ctx, session.User); err != nil || total != 0 {
		t.Fatalf("unexpected Stat: %d, err: %v", total, err)
	}

	// Broadcast and Publish with an empty dispatcher return zero.
	if total, err := p.Broadcast(ctx, &cluster.BroadcastArgs{Message: &cluster.Message{}}); err != nil || total != 0 {
		t.Fatalf("unexpected Broadcast: %d, err: %v", total, err)
	}
	if total, err := p.Publish(ctx, &cluster.PublishArgs{Message: &cluster.Message{}}); err != nil || total != 0 {
		t.Fatalf("unexpected Publish: %d, err: %v", total, err)
	}

	// The remaining delegating calls must fail without panicking.
	_, _ = p.GetIP(ctx, &cluster.GetIPArgs{Kind: session.User})
	_, _ = p.IsOnline(ctx, &cluster.IsOnlineArgs{Kind: session.Conn})
	_ = p.Disconnect(ctx, &cluster.DisconnectArgs{Kind: session.Conn})
	_ = p.Push(ctx, &cluster.PushArgs{Kind: session.Conn, Message: &cluster.Message{}})
	_, _ = p.Multicast(ctx, &cluster.MulticastArgs{Kind: session.Conn, Targets: []int64{1}, Message: &cluster.Message{}})
	_ = p.Subscribe(ctx, &cluster.SubscribeArgs{Kind: session.Conn, Targets: []int64{1}})
	_ = p.Unsubscribe(ctx, &cluster.UnsubscribeArgs{Kind: session.Conn, Targets: []int64{1}})
	_ = p.Deliver(ctx, &cluster.DeliverArgs{NID: "missing", Message: &cluster.Message{Data: []byte("x")}})

	// Setting the state delegates to the mesh.
	f.mesh.instance = &registry.ServiceInstance{ID: "test"}
	if err := p.SetState(cluster.Busy); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	// Adding hooks and providers while working is ignored.
	p.AddHookListener(cluster.Start, func(p *Proxy) {})
	p.AddServiceProvider("late", "desc", &struct{}{})

	// Watching must start every linker watch loop without blocking.
	p.watch()
}
