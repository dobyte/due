package mesh

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
)

// HookHandler is the hook handler of the mesh.
type HookHandler func(proxy *Proxy)

// Mesh is the mesh server.
type Mesh struct {
	component.Base
	opts        *options
	ctx         context.Context
	cancel      context.CancelFunc
	state       atomic.Int32
	proxy       *Proxy
	transporter transport.Server
	services    []*serviceEntity
	instance    *registry.ServiceInstance
	rw          sync.RWMutex
	hooks       map[cluster.Hook][]HookHandler
}

// serviceEntity is a service entity.
type serviceEntity struct {
	name     string // service name, used for service discovery
	desc     any    // service description (a desc object for grpc, a service path for rpcx)
	provider any    // service provider
}

// NewMesh returns a new mesh server.
//
// It initializes the proxy and internal components and stays in the shut state.
func NewMesh(opts ...Option) *Mesh {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	m := &Mesh{}
	m.opts = o
	m.hooks = make(map[cluster.Hook][]HookHandler)
	m.services = make([]*serviceEntity, 0)
	m.ctx, m.cancel = context.WithCancel(o.ctx)
	m.proxy = newProxy(m)
	m.state.Store(int32(cluster.Shut))

	return m
}

// Name returns the component name.
func (m *Mesh) Name() string {
	return m.opts.name
}

// Init initializes the mesh.
//
// It validates required configuration such as the codec, registry and transporter, terminating the
// process when they are missing.
func (m *Mesh) Init() {
	if m.opts.codec == nil {
		log.Fatal("codec component is not injected")
	}

	if m.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}

	if m.opts.transporter == nil {
		log.Fatal("transporter component is not injected")
	}

	m.runHookFunc(cluster.Init)
}

// Start starts the mesh.
//
// It marks the mesh as working, then starts the transport server, registers the service instance
// and starts watching.
func (m *Mesh) Start() {
	if !m.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	m.startTransportServer()

	m.registerServiceInstance()

	m.proxy.watch()

	m.printInfo()

	m.runHookFunc(cluster.Start)
}

// Close closes the mesh.
//
// It marks the mesh as hang and refreshes the service instance state in the registry.
func (m *Mesh) Close() {
	if !m.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !m.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	m.refreshServiceInstance()

	m.runHookFunc(cluster.Close)
}

// Destroy destroys the mesh server.
//
// It marks the mesh as shut, deregisters the service instance, stops the transport server and
// releases the internal component resources.
func (m *Mesh) Destroy() {
	if !m.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	m.deregisterServiceInstance()

	m.stopTransportServer()

	m.cancel()

	m.runHookFunc(cluster.Destroy)
}

// Proxy returns the mesh proxy.
func (m *Mesh) Proxy() *Proxy {
	return m.proxy
}

// startTransportServer starts the transport server.
//
// It sets the default service discovery and registers the service providers, terminating the
// process when no service provider exists.
func (m *Mesh) startTransportServer() {
	if len(m.services) == 0 {
		log.Fatal("no service registered")
	}

	m.opts.transporter.SetDefaultDiscovery(m.opts.registry)

	transporter, err := m.opts.transporter.NewServer()
	if err != nil {
		log.Fatalf("transport server create failed: %v", err)
	}

	m.transporter = transporter

	for _, entity := range m.services {
		if err = m.transporter.RegisterService(entity.desc, entity.provider); err != nil {
			log.Fatalf("register service failed: %v", err)
		}
	}

	go func() {
		if err = m.transporter.Start(); err != nil {
			log.Fatalf("transport server start failed: %v", err)
		}
	}()
}

// stopTransportServer stops the transport server.
func (m *Mesh) stopTransportServer() {
	if m.transporter == nil {
		return
	}

	if err := m.transporter.Stop(); err != nil {
		log.Errorf("transport server stop failed: %v", err)
	}
}

// registerServiceInstance registers the service instance.
//
// It builds the mesh service instance and registers it with the registry.
func (m *Mesh) registerServiceInstance() {
	m.instance = &registry.ServiceInstance{
		ID:       m.opts.id,
		Name:     cluster.Mesh.String(),
		Kind:     cluster.Mesh.String(),
		Alias:    m.opts.name,
		State:    m.getState().String(),
		Endpoint: m.transporter.Endpoint().String(),
		Services: make([]string, 0, len(m.services)),
		Weight:   m.opts.weight,
		Metadata: m.opts.metadata,
	}

	for _, item := range m.services {
		m.instance.Services = append(m.instance.Services, item.name)
	}

	if err := m.doRegisterServiceInstance(); err != nil {
		log.Fatalf("register cluster instance failed: %v", err)
	}
}

// refreshServiceInstance re-registers the service instance in the registry with the current state.
func (m *Mesh) refreshServiceInstance() {
	if err := m.doRefreshServiceInstance(m.getState()); err != nil {
		log.Errorf("refresh cluster instance failed: %v", err)
	}
}

// deregisterServiceInstance deregisters the service instance.
func (m *Mesh) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
	err := m.opts.registry.Deregister(ctx, m.instance)
	cancel()
	if err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// doRegisterServiceInstance registers the service instance, returning an error when registration
// fails.
func (m *Mesh) doRegisterServiceInstance() error {
	ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
	err := m.opts.registry.Register(ctx, m.instance)
	cancel()

	return err
}

// doRefreshServiceInstance refreshes the instance state, returning an error when registration
// fails. When state is omitted, the instance is only re-registered without updating its state.
func (m *Mesh) doRefreshServiceInstance(state ...cluster.State) error {
	if len(state) > 0 {
		m.instance.State = state[0].String()
	}

	return m.doRegisterServiceInstance()
}

// runHookFunc runs every listener of the given hook and waits for all of them to finish.
func (m *Mesh) runHookFunc(hook cluster.Hook) {
	m.rw.RLock()

	if handlers, ok := m.hooks[hook]; ok {
		wg := &sync.WaitGroup{}
		wg.Add(len(handlers))

		for i := range handlers {
			handler := handlers[i]
			xcall.Go(func() {
				handler(m.proxy)
				wg.Done()
			})
		}

		m.rw.RUnlock()

		wg.Wait()
	} else {
		m.rw.RUnlock()
	}
}

// addHookListener adds a hook listener for the given hook.
func (m *Mesh) addHookListener(hook cluster.Hook, handler HookHandler) {
	switch hook {
	case cluster.Destroy:
		m.rw.Lock()
		m.hooks[hook] = append(m.hooks[hook], handler)
		m.rw.Unlock()
	default:
		if m.getState() == cluster.Shut {
			m.rw.Lock()
			m.hooks[hook] = append(m.hooks[hook], handler)
			m.rw.Unlock()
		} else {
			log.Warnf("server is working, can't add hook handler")
		}
	}
}

// addServiceProvider adds a service provider. The name is the service name, desc is the service
// description and provider is the service provider.
func (m *Mesh) addServiceProvider(name string, desc, provider any) {
	if m.getState() == cluster.Shut {
		m.services = append(m.services, &serviceEntity{
			name:     name,
			desc:     desc,
			provider: provider,
		})
	} else {
		log.Warnf("mesh server is working, can't add service provider")
	}
}

// getState returns the current mesh state.
func (m *Mesh) getState() cluster.State {
	return cluster.State(m.state.Load())
}

// setState updates the state, which may only switch between Work and Busy. On success it refreshes
// the service instance state in the registry. Only Work and Busy are supported; an error is
// returned when the state is illegal, the switch fails or the refresh fails.
func (m *Mesh) setState(state cluster.State) error {
	if state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := m.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if m.state.CompareAndSwap(int32(curr), int32(state)) {
			return m.doRefreshServiceInstance(state)
		} else {
			return errors.ErrIllegalOperation
		}
	default:
		return errors.ErrIllegalOperation
	}
}

// isShut reports whether the mesh is in the shut state.
func (m *Mesh) isShut() bool {
	return m.getState() == cluster.Shut
}

// printInfo prints basic information such as the mesh ID, name, codec, locator, registry and
// transporter.
func (m *Mesh) printInfo() {
	rows := make([]string, 0, 7)
	rows = append(rows, fmt.Sprintf("ID: %s", m.opts.id))
	rows = append(rows, fmt.Sprintf("Name: %s", m.Name()))
	rows = append(rows, fmt.Sprintf("Codec: %s", m.opts.codec.Name()))

	if m.opts.locator != nil {
		rows = append(rows, fmt.Sprintf("Locator: %s", m.opts.locator.Name()))
	} else {
		rows = append(rows, "Locator: -")
	}

	rows = append(rows, fmt.Sprintf("Registry: %s", m.opts.registry.Name()))

	if m.opts.encryptor != nil {
		rows = append(rows, fmt.Sprintf("Encryptor: %s", m.opts.encryptor.Name()))
	} else {
		rows = append(rows, "Encryptor: -")
	}

	rows = append(rows, fmt.Sprintf("Transporter: %s", m.opts.transporter.Name()))

	info.Print("Mesh", rows...)
}
