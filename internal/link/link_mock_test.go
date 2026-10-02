package link

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
)

// gateProviderStub is a gate.Provider test double whose behaviour can be overridden per method.
// Push-family methods release the received buffer so that reference counting stays balanced.
type gateProviderStub struct {
	bindFn        func(ctx context.Context, cid, uid int64) error
	unbindFn      func(ctx context.Context, uid int64) error
	getIPFn       func(ctx context.Context, kind session.Kind, target int64) (string, error)
	isOnlineFn    func(ctx context.Context, kind session.Kind, target int64) (bool, error)
	statFn        func(ctx context.Context, kind session.Kind) (int64, error)
	disconnectFn  func(ctx context.Context, kind session.Kind, target int64, force bool) error
	pushFn        func(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer) error
	multicastFn   func(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer) (int64, error)
	broadcastFn   func(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer) (int64, error)
	publishFn     func(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer) (int64, error)
	subscribeFn   func(ctx context.Context, kind session.Kind, targets []int64, channel string) error
	unsubscribeFn func(ctx context.Context, kind session.Kind, targets []int64, channel string) error
	getStateFn    func() (cluster.State, error)
	setStateFn    func(state cluster.State) error

	bindCount        atomic.Int64
	unbindCount      atomic.Int64
	getIPCount       atomic.Int64
	isOnlineCount    atomic.Int64
	statCount        atomic.Int64
	disconnectCount  atomic.Int64
	pushCount        atomic.Int64
	multicastCount   atomic.Int64
	broadcastCount   atomic.Int64
	publishCount     atomic.Int64
	subscribeCount   atomic.Int64
	unsubscribeCount atomic.Int64

	mu         sync.Mutex
	lastCID    int64
	lastUID    int64
	lastTarget int64
	lastForce  bool
	lastKind   session.Kind
	lastChan   string
	lastState  cluster.State
}

// Bind binds the relationship between the user and the gate.
func (p *gateProviderStub) Bind(ctx context.Context, cid, uid int64) error {
	p.bindCount.Add(1)
	p.mu.Lock()
	p.lastCID, p.lastUID = cid, uid
	p.mu.Unlock()
	if p.bindFn != nil {
		return p.bindFn(ctx, cid, uid)
	}
	return nil
}

// Unbind unbinds the relationship between the user and the gate.
func (p *gateProviderStub) Unbind(ctx context.Context, uid int64) error {
	p.unbindCount.Add(1)
	if p.unbindFn != nil {
		return p.unbindFn(ctx, uid)
	}
	return nil
}

// GetIP returns the client IP address.
func (p *gateProviderStub) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	p.getIPCount.Add(1)
	if p.getIPFn != nil {
		return p.getIPFn(ctx, kind, target)
	}
	return "127.0.0.1", nil
}

// IsOnline reports whether the target is online.
func (p *gateProviderStub) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	p.isOnlineCount.Add(1)
	if p.isOnlineFn != nil {
		return p.isOnlineFn(ctx, kind, target)
	}
	return true, nil
}

// Stat returns the total number of sessions.
func (p *gateProviderStub) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	p.statCount.Add(1)
	if p.statFn != nil {
		return p.statFn(ctx, kind)
	}
	return 7, nil
}

// Disconnect disconnects the target session.
func (p *gateProviderStub) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	p.disconnectCount.Add(1)
	p.mu.Lock()
	p.lastKind, p.lastTarget, p.lastForce = kind, target, force
	p.mu.Unlock()
	if p.disconnectFn != nil {
		return p.disconnectFn(ctx, kind, target, force)
	}
	return nil
}

// Push sends a single message.
func (p *gateProviderStub) Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer) error {
	p.pushCount.Add(1)
	p.mu.Lock()
	p.lastKind, p.lastTarget = kind, target
	p.mu.Unlock()
	var err error
	if p.pushFn != nil {
		err = p.pushFn(ctx, kind, target, disconnect, buf)
	}
	buf.Release()
	return err
}

// Multicast pushes a multicast message.
func (p *gateProviderStub) Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer) (int64, error) {
	p.multicastCount.Add(1)
	var (
		total int64 = int64(len(targets))
		err   error
	)
	if p.multicastFn != nil {
		total, err = p.multicastFn(ctx, kind, targets, disconnect, buf)
	}
	buf.Release()
	return total, err
}

// Broadcast pushes a broadcast message.
func (p *gateProviderStub) Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer) (int64, error) {
	p.broadcastCount.Add(1)
	var (
		total int64 = 3
		err   error
	)
	if p.broadcastFn != nil {
		total, err = p.broadcastFn(ctx, kind, disconnect, buf)
	}
	buf.Release()
	return total, err
}

// Publish publishes a channel message.
func (p *gateProviderStub) Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer) (int64, error) {
	p.publishCount.Add(1)
	p.mu.Lock()
	p.lastChan = channel
	p.mu.Unlock()
	var (
		total int64 = 2
		err   error
	)
	if p.publishFn != nil {
		total, err = p.publishFn(ctx, channel, disconnect, buf)
	}
	buf.Release()
	return total, err
}

// Subscribe subscribes the targets to a channel.
func (p *gateProviderStub) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	p.subscribeCount.Add(1)
	p.mu.Lock()
	p.lastKind, p.lastChan = kind, channel
	p.mu.Unlock()
	if p.subscribeFn != nil {
		return p.subscribeFn(ctx, kind, targets, channel)
	}
	return nil
}

// Unsubscribe unsubscribes the targets from a channel.
func (p *gateProviderStub) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	p.unsubscribeCount.Add(1)
	p.mu.Lock()
	p.lastKind, p.lastChan = kind, channel
	p.mu.Unlock()
	if p.unsubscribeFn != nil {
		return p.unsubscribeFn(ctx, kind, targets, channel)
	}
	return nil
}

// GetState returns the state.
func (p *gateProviderStub) GetState() (cluster.State, error) {
	if p.getStateFn != nil {
		return p.getStateFn()
	}
	return cluster.Work, nil
}

// SetState sets the state.
func (p *gateProviderStub) SetState(state cluster.State) error {
	p.mu.Lock()
	p.lastState = state
	p.mu.Unlock()
	if p.setStateFn != nil {
		return p.setStateFn(state)
	}
	return nil
}

// nodeLinkDeliverRecord records one message delivered to the node provider stub.
type nodeLinkDeliverRecord struct {
	gid  string
	nid  string
	cid  int64
	uid  int64
	data []byte
}

// nodeProviderStub is a node.Provider test double that records messages delivered to it.
type nodeProviderStub struct {
	deliver chan nodeLinkDeliverRecord
	trigger chan nodeLinkTriggerRecord

	deliverFn   func(ctx context.Context, gid, nid string, cid, uid int64, buf buffer.Buffer) error
	triggerFn   func(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error
	getStateFn  func() (cluster.State, error)
	setStateFn  func(state cluster.State) error
	setStateVal atomic.Int64
}

// nodeLinkTriggerRecord records one event trigger.
type nodeLinkTriggerRecord struct {
	gid   string
	cid   int64
	uid   int64
	event cluster.Event
}

// Trigger triggers an event on the node.
func (p *nodeProviderStub) Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error {
	if p.trigger != nil {
		p.trigger <- nodeLinkTriggerRecord{gid: gid, cid: cid, uid: uid, event: event}
	}
	if p.triggerFn != nil {
		return p.triggerFn(ctx, gid, cid, uid, event)
	}
	return nil
}

// Deliver delivers a message to the node.
func (p *nodeProviderStub) Deliver(ctx context.Context, gid, nid string, cid, uid int64, buf buffer.Buffer) error {
	if p.deliver != nil {
		// Copy the bytes: the buffer may be recycled by the reader goroutine once released.
		data := append([]byte(nil), buf.Bytes()...)
		p.deliver <- nodeLinkDeliverRecord{gid: gid, nid: nid, cid: cid, uid: uid, data: data}
	}
	var err error
	if p.deliverFn != nil {
		err = p.deliverFn(ctx, gid, nid, cid, uid, buf)
	}
	buf.Release()
	return err
}

// GetState returns the state.
func (p *nodeProviderStub) GetState() (cluster.State, error) {
	if p.getStateFn != nil {
		return p.getStateFn()
	}
	return cluster.Work, nil
}

// SetState sets the state.
func (p *nodeProviderStub) SetState(state cluster.State) error {
	p.setStateVal.Store(int64(state))
	if p.setStateFn != nil {
		return p.setStateFn(state)
	}
	return nil
}

// locatorStub is a locate.Locator test double whose behaviour can be overridden per method.
type locatorStub struct {
	watchFn       func(ctx context.Context, kinds ...string) (locate.Watcher, error)
	bindGateFn    func(ctx context.Context, uid int64, gid string) error
	bindNodeFn    func(ctx context.Context, uid int64, name, nid string) error
	unbindGateFn  func(ctx context.Context, uid int64, gid string) error
	unbindNodeFn  func(ctx context.Context, uid int64, name, nid string) error
	locateGateFn  func(ctx context.Context, uid int64) (string, error)
	locateNodeFn  func(ctx context.Context, uid int64, name string) (string, error)
	locateNodesFn func(ctx context.Context, uid int64) (map[string]string, error)
	closeFn       func() error
}

// Name returns the component name of the locator.
func (s *locatorStub) Name() string { return "stub" }

// Watch watches changes to user locations.
func (s *locatorStub) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	if s.watchFn != nil {
		return s.watchFn(ctx, kinds...)
	}
	return &locateWatcherStub{steps: make(chan locateWatchStep, 1)}, nil
}

// BindGate binds the user to the gate identified by gid.
func (s *locatorStub) BindGate(ctx context.Context, uid int64, gid string) error {
	if s.bindGateFn != nil {
		return s.bindGateFn(ctx, uid, gid)
	}
	return nil
}

// BindNode binds the user to the node identified by name and nid.
func (s *locatorStub) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if s.bindNodeFn != nil {
		return s.bindNodeFn(ctx, uid, name, nid)
	}
	return nil
}

// UnbindGate unbinds the user from the gate identified by gid.
func (s *locatorStub) UnbindGate(ctx context.Context, uid int64, gid string) error {
	if s.unbindGateFn != nil {
		return s.unbindGateFn(ctx, uid, gid)
	}
	return nil
}

// UnbindNode unbinds the user from the node identified by name and nid.
func (s *locatorStub) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if s.unbindNodeFn != nil {
		return s.unbindNodeFn(ctx, uid, name, nid)
	}
	return nil
}

// LocateGate locates the gate the user is bound to.
func (s *locatorStub) LocateGate(ctx context.Context, uid int64) (string, error) {
	if s.locateGateFn != nil {
		return s.locateGateFn(ctx, uid)
	}
	return "", nil
}

// LocateNode locates the node the user is bound to under the given name.
func (s *locatorStub) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if s.locateNodeFn != nil {
		return s.locateNodeFn(ctx, uid, name)
	}
	return "", nil
}

// LocateNodes locates every node the user is bound to.
func (s *locatorStub) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if s.locateNodesFn != nil {
		return s.locateNodesFn(ctx, uid)
	}
	return nil, nil
}

// Close closes the locator.
func (s *locatorStub) Close() error {
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

// locateWatchStep is one scripted step of the locate watcher.
type locateWatchStep struct {
	events []*locate.Event
	err    error
}

// locateWatcherStub replays scripted locate events and reports ErrWatcherStopped once exhausted.
type locateWatcherStub struct {
	steps chan locateWatchStep
	once  sync.Once
}

// Next returns the next batch of user location events.
func (w *locateWatcherStub) Next() ([]*locate.Event, error) {
	step, ok := <-w.steps
	if !ok {
		return nil, errors.ErrWatcherStopped
	}
	return step.events, step.err
}

// Stop stops watching.
func (w *locateWatcherStub) Stop() error {
	w.once.Do(func() { close(w.steps) })
	return nil
}

// registryStub is a registry.Registry test double whose behaviour can be overridden per method.
type registryStub struct {
	servicesFn   func(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error)
	watchFn      func(ctx context.Context, serviceName string) (registry.Watcher, error)
	registerFn   func(ctx context.Context, ins *registry.ServiceInstance) error
	deregisterFn func(ctx context.Context, ins *registry.ServiceInstance) error
	closeFn      func() error
}

// Name returns the name of the service registry and discovery component.
func (s *registryStub) Name() string { return "stub" }

// Register registers a service instance.
func (s *registryStub) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if s.registerFn != nil {
		return s.registerFn(ctx, ins)
	}
	return nil
}

// Deregister deregisters a service instance.
func (s *registryStub) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if s.deregisterFn != nil {
		return s.deregisterFn(ctx, ins)
	}
	return nil
}

// Watch watches for changes to the service instances with the same service name.
func (s *registryStub) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if s.watchFn != nil {
		return s.watchFn(ctx, serviceName)
	}
	return &registryWatcherStub{steps: make(chan registryWatchStep, 1)}, nil
}

// Services returns the list of service instances.
func (s *registryStub) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if s.servicesFn != nil {
		return s.servicesFn(ctx, serviceName)
	}
	return nil, nil
}

// Close closes the service registry and discovery component.
func (s *registryStub) Close() error {
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

// registryWatchStep is one scripted step of the registry watcher.
type registryWatchStep struct {
	services []*registry.ServiceInstance
	err      error
}

// registryWatcherStub replays scripted service instances and reports context.Canceled once
// exhausted.
type registryWatcherStub struct {
	steps chan registryWatchStep
	once  sync.Once
}

// Next returns the next list of service instances.
func (w *registryWatcherStub) Next() ([]*registry.ServiceInstance, error) {
	step, ok := <-w.steps
	if !ok {
		return nil, context.Canceled
	}
	return step.services, step.err
}

// Stop stops watching.
func (w *registryWatcherStub) Stop() error {
	w.once.Do(func() { close(w.steps) })
	return nil
}

// codecStub is an encoding.Codec test double.
type codecStub struct {
	marshalFn   func(v any) ([]byte, error)
	unmarshalFn func(data []byte, v any) error
}

// Name returns the codec name.
func (c *codecStub) Name() string { return "stub" }

// Marshal encodes v.
func (c *codecStub) Marshal(v any) ([]byte, error) {
	if c.marshalFn != nil {
		return c.marshalFn(v)
	}
	return []byte("stub"), nil
}

// Unmarshal decodes data into v.
func (c *codecStub) Unmarshal(data []byte, v any) error {
	if c.unmarshalFn != nil {
		return c.unmarshalFn(data, v)
	}
	return nil
}

// encryptorStub is a crypto.Encryptor test double that prefixes the payload.
type encryptorStub struct{}

// Name returns the name of the encryptor.
func (encryptorStub) Name() string { return "stub" }

// Encrypt prefixes data with the marker "enc:".
func (encryptorStub) Encrypt(data []byte) ([]byte, error) {
	return append([]byte("enc:"), data...), nil
}

// Decrypt strips the marker added by Encrypt.
func (encryptorStub) Decrypt(data []byte) ([]byte, error) { return data, nil }

// baseLinkOptions returns linker options shared by the tests, pointing at in-process servers.
func baseLinkOptions() *Options {
	return &Options{
		ID:                  "test-gate",
		Kind:                cluster.Gate,
		Codec:               json.DefaultCodec,
		Dispatch:            cluster.Random,
		ConnNum:             1,
		CallTimeout:         2 * time.Second,
		DialTimeout:         200 * time.Millisecond,
		DialRetryTimes:      1,
		FaultRecoveryTime:   5 * time.Second,
		CommandQueueSize:    128,
		CommandWriteTimeout: time.Second,
	}
}

// newGateLinker returns a GateLinker backed by the base options and a stub locator. The locator
// allows cached user sources to be resolved, mirroring a configured locator in production.
func newGateLinker() *GateLinker {
	opts := baseLinkOptions()
	opts.Locator = &locatorStub{}
	return NewGateLinker(context.Background(), opts)
}

// startGateServer starts an in-process gate transporter server on a random local port. The server
// is stopped automatically when the test finishes.
func startGateServer(t *testing.T, p gate.Provider) (*gate.Server, string) {
	t.Helper()

	addr := listenFreeAddr(t)

	server, err := gate.NewServer(p, &gate.ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create gate server failed: %v", err)
	}

	if err = server.Start(); err != nil {
		t.Fatalf("start gate server failed: %v", err)
	}

	t.Cleanup(func() { _ = server.Stop() })

	return server, server.ListenAddr()
}

// startNodeServer starts an in-process node transporter server on a random local port. The server
// is stopped automatically when the test finishes.
func startNodeServer(t *testing.T, p node.Provider) (*node.Server, string) {
	t.Helper()

	addr := listenFreeAddr(t)

	server, err := node.NewServer(p, &node.ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create node server failed: %v", err)
	}

	if err = server.Start(); err != nil {
		t.Fatalf("start node server failed: %v", err)
	}

	t.Cleanup(func() { _ = server.Stop() })

	return server, server.ListenAddr()
}

// gateService builds a gate service instance reachable at addr.
func gateService(id, addr string) *registry.ServiceInstance {
	return &registry.ServiceInstance{
		ID:       id,
		Name:     cluster.Gate.String(),
		Kind:     cluster.Gate.String(),
		State:    cluster.Work.String(),
		Endpoint: "drpc://" + addr,
	}
}

// nodeService builds a node service instance reachable at addr with the given routes.
func nodeService(id, addr string, routes ...registry.Route) *registry.ServiceInstance {
	return &registry.ServiceInstance{
		ID:       id,
		Name:     cluster.Node.String(),
		Kind:     cluster.Node.String(),
		Alias:    id,
		State:    cluster.Work.String(),
		Endpoint: "drpc://" + addr,
		Routes:   routes,
	}
}

// waitFor polls cond until it reports true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}

	return cond()
}
