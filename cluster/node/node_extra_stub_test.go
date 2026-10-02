package node

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

// stubLocator is a minimal locate.Locator implementation for tests. Its watcher blocks until the
// done channel is closed, which keeps the watch goroutines idle instead of busy-looping.
type stubLocator struct {
	gid       string
	nid       string
	watchDone chan struct{}

	bindGateCount   int
	bindNodeCount   int
	unbindGateCount int
	unbindNodeCount int

	mu sync.Mutex
}

func (l *stubLocator) Name() string { return "stub-locator" }

func (l *stubLocator) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	return &stubLocatorWatcher{done: l.watchDone}, nil
}

func (l *stubLocator) BindGate(ctx context.Context, uid int64, gid string) error {
	l.mu.Lock()
	l.bindGateCount++
	l.mu.Unlock()

	return nil
}

func (l *stubLocator) BindNode(ctx context.Context, uid int64, name, nid string) error {
	l.mu.Lock()
	l.bindNodeCount++
	l.mu.Unlock()

	return nil
}

func (l *stubLocator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	l.mu.Lock()
	l.unbindGateCount++
	l.mu.Unlock()

	return nil
}

func (l *stubLocator) UnbindNode(ctx context.Context, uid int64, name string, nid string) error {
	l.mu.Lock()
	l.unbindNodeCount++
	l.mu.Unlock()

	return nil
}

func (l *stubLocator) LocateGate(ctx context.Context, uid int64) (string, error) {
	return l.gid, nil
}

func (l *stubLocator) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return l.nid, nil
}

func (l *stubLocator) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	return map[string]string{"node": l.nid}, nil
}

func (l *stubLocator) Close() error { return nil }

// stubLocatorWatcher is a locator watcher that blocks until done is closed.
type stubLocatorWatcher struct {
	done <-chan struct{}
}

func (w *stubLocatorWatcher) Next() ([]*locate.Event, error) {
	<-w.done

	return nil, errors.ErrWatcherStopped
}

func (w *stubLocatorWatcher) Stop() error { return nil }

// stubRegistry is a minimal registry.Registry implementation for tests.
type stubRegistry struct {
	watchDone     chan struct{}
	registerErr   error
	deregisterErr error

	mu               sync.Mutex
	registerCount    int
	deregisterCount  int
	lastRegistered   *registry.ServiceInstance
	lastDeregistered *registry.ServiceInstance
}

func (r *stubRegistry) Name() string { return "stub-registry" }

func (r *stubRegistry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	r.mu.Lock()
	r.registerCount++
	r.lastRegistered = ins
	r.mu.Unlock()

	return r.registerErr
}

func (r *stubRegistry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	r.mu.Lock()
	r.deregisterCount++
	r.lastDeregistered = ins
	r.mu.Unlock()

	return r.deregisterErr
}

func (r *stubRegistry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	return &stubRegistryWatcher{done: r.watchDone}, nil
}

func (r *stubRegistry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	return nil, nil
}

func (r *stubRegistry) Close() error { return nil }

// stubRegistryWatcher is a registry watcher that blocks until done is closed.
type stubRegistryWatcher struct {
	done <-chan struct{}
}

func (w *stubRegistryWatcher) Next() ([]*registry.ServiceInstance, error) {
	<-w.done

	return nil, context.Canceled
}

func (w *stubRegistryWatcher) Stop() error { return nil }

// stubEncryptor is a minimal crypto.Encryptor implementation for tests.
type stubEncryptor struct {
	encryptErr error
	decryptErr error
}

func (e *stubEncryptor) Name() string { return "stub-encryptor" }

func (e *stubEncryptor) Encrypt(data []byte) ([]byte, error) {
	if e.encryptErr != nil {
		return nil, e.encryptErr
	}

	return data, nil
}

func (e *stubEncryptor) Decrypt(data []byte) ([]byte, error) {
	if e.decryptErr != nil {
		return nil, e.decryptErr
	}

	return data, nil
}

// stubTransportServer is a minimal transport.Server implementation for tests.
type stubTransportServer struct {
	addr       string
	registered int
}

func (s *stubTransportServer) Start() error { return nil }

func (s *stubTransportServer) Stop() error { return nil }

func (s *stubTransportServer) Addr() string { return s.addr }

func (s *stubTransportServer) Scheme() string { return "stub" }

func (s *stubTransportServer) Endpoint() *endpoint.Endpoint {
	return endpoint.NewEndpoint("stub", s.addr, false)
}

func (s *stubTransportServer) RegisterService(desc, service any) error {
	s.registered++

	return nil
}

// stubTransporter is a minimal transport.Transporter implementation for tests.
type stubTransporter struct {
	server     *stubTransportServer
	client     transport.Client
	discovery  registry.Discovery
	closeCount int
}

func (t *stubTransporter) Name() string { return "stub-transporter" }

func (t *stubTransporter) NewServer() (transport.Server, error) {
	if t.server == nil {
		t.server = &stubTransportServer{addr: "127.0.0.1:0"}
	}

	return t.server, nil
}

func (t *stubTransporter) NewClient(target string) (transport.Client, error) {
	if t.client != nil {
		return t.client, nil
	}

	return nil, errors.ErrClientShut
}

func (t *stubTransporter) SetDefaultDiscovery(discovery registry.Discovery) {
	t.discovery = discovery
}

func (t *stubTransporter) Close() error {
	t.closeCount++

	return nil
}

// stubCodec wraps the json codec so that encode/decode failures can be injected in tests.
type stubCodec struct {
	inner        encoding.Codec
	marshalErr   error
	unmarshalErr error
}

func newStubCodec() *stubCodec {
	return &stubCodec{inner: encoding.Invoke("json")}
}

func (c *stubCodec) Name() string { return c.inner.Name() }

func (c *stubCodec) Marshal(v any) ([]byte, error) {
	if c.marshalErr != nil {
		return nil, c.marshalErr
	}

	return c.inner.Marshal(v)
}

func (c *stubCodec) Unmarshal(data []byte, v any) error {
	if c.unmarshalErr != nil {
		return c.unmarshalErr
	}

	return c.inner.Unmarshal(data, v)
}

// stubNode creates a node backed by stubs so that no external service is required, and returns the
// stubs as well so that tests can inspect and tweak them. The stub watchers are always released at
// the end of the test.
func stubNode(t *testing.T, opts ...Option) (*Node, *stubLocator, *stubRegistry) {
	t.Helper()

	locator := &stubLocator{gid: "gate-1", nid: "node-2", watchDone: make(chan struct{})}
	registry := &stubRegistry{watchDone: make(chan struct{})}

	t.Cleanup(func() {
		close(locator.watchDone)
		close(registry.watchDone)
	})

	base := []Option{
		WithID("test-node-id"),
		WithName("test-node-name"),
		WithAddr("127.0.0.1:0"),
		WithCodec(encoding.Invoke("json")),
		WithLocator(locator),
		WithRegistry(registry),
	}

	return NewNode(append(base, opts...)...), locator, registry
}

// newTestNode creates a node backed by stubs and discards the stub references.
func newTestNode(t *testing.T, opts ...Option) *Node {
	t.Helper()

	n, _, _ := stubNode(t, opts...)

	return n
}

// setWorking forces a node into the work state without starting its servers, so that the code
// paths guarded by isShut can be exercised without any network dependency.
func setWorking(n *Node) {
	n.state.Store(int32(cluster.Work))
}

// newTestNodeCtx returns a plain background context for option tests.
func newTestNodeCtx() context.Context {
	return context.Background()
}

// stubProcessor is a test actor processor that counts its lifecycle callbacks.
type stubProcessor struct {
	initCount    atomic.Int32
	startCount   atomic.Int32
	destroyCount atomic.Int32
}

func (p *stubProcessor) Init() { p.initCount.Add(1) }

func (p *stubProcessor) Start() { p.startCount.Add(1) }

func (p *stubProcessor) Destroy() { p.destroyCount.Add(1) }

// newStubProcessorCreator returns a creator producing stub processors.
func newStubProcessorCreator() Creator {
	return func(actor *Actor, args ...any) Processor {
		return &stubProcessor{}
	}
}
