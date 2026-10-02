package node

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/petermattis/goid"
	"golang.org/x/sync/errgroup"
)

// HookHandler is the node hook handler.
type HookHandler func(proxy *Proxy)

// serviceEntity is a service entity.
type serviceEntity struct {
	name     string // Service name, used for service discovery
	desc     any    // Service description (the desc object for grpc, the service path for rpcx)
	provider any    // Service provider
}

// Node is a node server.
type Node struct {
	component.Base
	opts         *options
	ctx          context.Context
	cancel       context.CancelFunc
	state        atomic.Int32
	destroyed    atomic.Bool
	evtPool      *sync.Pool
	reqPool      *sync.Pool
	tasker       *queue.Tasker
	router       *Router
	trigger      *Trigger
	proxy        *Proxy
	services     []*serviceEntity
	instances    []*registry.ServiceInstance
	linker       *node.Server
	scheduler    *Scheduler
	transporter  transport.Server
	rw           sync.RWMutex
	hooks        map[cluster.Hook][]HookHandler
	dispatchGoid atomic.Int64
	counter      atomic.Int64
	done         chan struct{}
	wg           sync.WaitGroup
}

// NewNode creates a node server.
//
// It initializes internal components such as the proxy, tasker, router, trigger and scheduler, and
// starts in the shut state.
func NewNode(opts ...Option) *Node {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	n := &Node{}
	n.opts = o
	n.ctx, n.cancel = context.WithCancel(o.ctx)
	n.proxy = newProxy(n)
	n.tasker = queue.NewTasker(n.opts.taskQueueSize, n.opts.taskWriteTimeout, &sync.RWMutex{})
	n.router = newRouter(n)
	n.trigger = newTrigger(n)
	n.scheduler = newScheduler(n)
	n.hooks = make(map[cluster.Hook][]HookHandler)
	n.services = make([]*serviceEntity, 0)
	n.instances = make([]*registry.ServiceInstance, 0)
	n.state.Store(int32(cluster.Shut))
	n.evtPool = &sync.Pool{New: func() any { return &event{node: n} }}
	n.reqPool = &sync.Pool{New: func() any { return &request{node: n} }}
	n.done = make(chan struct{})

	return n
}

// Name returns the component name.
func (n *Node) Name() string {
	return n.opts.name
}

// Init initializes the node.
//
// It validates the required configuration such as the instance ID, instance name, codec, locator
// and registry, and terminates the process when any of them is missing.
func (n *Node) Init() {
	if n.opts.id == "" {
		log.Fatal("instance id can not be empty")
	}

	if n.opts.name == "" {
		log.Fatal("instance name can not be empty")
	}

	if n.opts.codec == nil {
		log.Fatal("codec component is not injected")
	}

	if n.opts.locator == nil {
		log.Fatal("locator component is not injected")
	}

	if n.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}

	n.runHookFunc(cluster.Init)
}

// Start starts the node.
//
// It sets the state to working, then starts the linker server and the transport server, registers
// the service instances and begins dispatching messages.
func (n *Node) Start() {
	if !n.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	n.startLinkerServer()

	n.startTransportServer()

	n.registerServiceInstances()

	n.proxy.watch()

	n.wg.Go(n.dispatch)

	n.printInfo()

	n.runHookFunc(cluster.Start)
}

// Close closes the node.
//
// It sets the state to hang, stops accepting new messages and waits for all in-flight asynchronous
// tasks to finish. The messages already in the route and event queues are not waited for and the
// dispatcher keeps processing them until Destroy; any residual messages not drained by Destroy are
// discarded.
func (n *Node) Close() {
	if !n.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !n.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	n.refreshServiceInstances(cluster.Hang)

	n.runHookFunc(cluster.Close)

	if n.counter.Load() <= 0 {
		if n.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
			close(n.done)
		}
	}

	<-n.done
}

// Destroy destroys the node server.
//
// It sets the state to shut, deregisters the service instances, stops the linker and transport
// servers and releases the resources of the internal components. Destroy is idempotent and repeated
// calls have no side effects.
func (n *Node) Destroy() {
	if !n.destroyed.CompareAndSwap(false, true) {
		return
	}

	// Release the Close waiters that may be interrupted by a timeout.
	if n.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		close(n.done)
	}

	n.deregisterServiceInstances()

	n.stopLinkerServer()

	n.stopTransportServer()

	n.tasker.Close()

	n.router.close()

	n.trigger.close()

	n.wg.Wait()

	n.tasker.Clean()

	n.router.clean()

	n.trigger.clean()

	n.runHookFunc(cluster.Destroy)

	n.cancel()
}

// Proxy returns the node proxy.
func (n *Node) Proxy() *Proxy {
	return n.proxy
}

// dispatch dispatches and handles messages.
//
// It loops over the messages received from the tasker, the router and the trigger and handles them
// one by one, exiting after all queues are closed and drained.
func (n *Node) dispatch() {
	n.dispatchGoid.Store(goid.Get())

	var (
		tasks    = n.tasker.Read()
		events   = n.trigger.receive()
		requests = n.router.receive()
	)

	for tasks != nil || requests != nil || events != nil {
		select {
		case tk, ok := <-tasks:
			if ok {
				n.tasker.Handle(tk, true)

				if tk != nil {
					n.doDoneWait()
				}
			} else {
				tasks = nil
			}
		case req, ok := <-requests:
			if ok {
				n.router.handle(req)
			} else {
				requests = nil
			}
		case evt, ok := <-events:
			if ok {
				n.trigger.handle(evt)
			} else {
				events = nil
			}
		}
	}
}

// startLinkerServer starts the linker server.
//
// It creates the internal linker server used to receive the events and messages delivered by the
// gateways and starts the service in a goroutine.
func (n *Node) startLinkerServer() {
	linker, err := node.NewServer(&provider{node: n}, &node.ServerOptions{
		Addr:   n.opts.addr,
		Expose: n.opts.expose,
	})
	if err != nil {
		log.Fatalf("linker server create failed: %v", err)
	}

	if err = linker.Start(); err != nil {
		log.Fatalf("linker server start failed: %v", err)
	}

	n.linker = linker
}

// stopLinkerServer stops the linker server.
func (n *Node) stopLinkerServer() {
	if n.linker == nil {
		return
	}

	if err := n.linker.Stop(); err != nil {
		log.Errorf("linker server stop failed: %v", err)
	}
}

// startTransportServer starts the transport server.
//
// It sets the default discovery and registers the service providers, and returns immediately when
// there is no service provider.
func (n *Node) startTransportServer() {
	if n.opts.transporter == nil {
		return
	}

	n.opts.transporter.SetDefaultDiscovery(n.opts.registry)

	if len(n.services) == 0 {
		return
	}

	transporter, err := n.opts.transporter.NewServer()
	if err != nil {
		log.Fatalf("transport server create failed: %v", err)
	}

	n.transporter = transporter

	for _, entity := range n.services {
		if err = n.transporter.RegisterService(entity.desc, entity.provider); err != nil {
			log.Fatalf("register service failed: %v", err)
		}
	}

	go func() {
		if err = n.transporter.Start(); err != nil {
			log.Fatalf("transport server start failed: %v", err)
		}
	}()
}

// stopTransportServer stops the transport server.
func (n *Node) stopTransportServer() {
	if n.transporter == nil {
		return
	}

	if err := n.transporter.Stop(); err != nil {
		log.Errorf("transport server stop failed: %v", err)
	}
}

// registerServiceInstances registers the service instances.
//
// It collects the route and event information to build the node and mesh service instances and
// registers them to the registry.
func (n *Node) registerServiceInstances() {
	routes := make([]registry.Route, 0, len(n.router.routes))
	events := make([]int, 0, len(n.trigger.events))

	for _, entity := range n.router.routes {
		routes = append(routes, registry.Route{
			ID:         entity.route,
			Internal:   entity.options.Internal,
			Stateful:   entity.options.Stateful,
			Authorized: entity.options.Authorized,
		})
	}

	for evt := range n.trigger.events {
		events = append(events, int(evt))
	}

	n.instances = append(n.instances, &registry.ServiceInstance{
		ID:       n.opts.id,
		Name:     cluster.Node.String(),
		Kind:     cluster.Node.String(),
		Alias:    n.opts.name,
		State:    n.getState().String(),
		Routes:   routes,
		Events:   events,
		Endpoint: n.linker.Endpoint().String(),
		Weight:   n.opts.weight,
		Metadata: n.opts.metadata,
	})

	if n.transporter != nil {
		services := make([]string, 0, len(n.services))
		for _, item := range n.services {
			services = append(services, item.name)
		}

		n.instances = append(n.instances, &registry.ServiceInstance{
			ID:       n.opts.id,
			Name:     cluster.Mesh.String(),
			Kind:     cluster.Mesh.String(),
			Alias:    n.opts.name,
			State:    n.getState().String(),
			Services: services,
			Endpoint: n.transporter.Endpoint().String(),
			Weight:   n.opts.weight,
			Metadata: n.opts.metadata,
		})
	}

	if err := n.doRegisterServiceInstances(); err != nil {
		log.Fatalf("register cluster instances failed: %v", err)
	}
}

// refreshServiceInstances refreshes the state of the service instances.
//
// The optional state is the new state of the service instances; when it is omitted the instances
// are only re-registered without updating the state.
func (n *Node) refreshServiceInstances(state ...cluster.State) {
	if err := n.doRefreshServiceInstances(state...); err != nil {
		log.Errorf("refresh cluster instances failed: %v", err)
	}
}

// deregisterServiceInstances deregisters the service instances.
//
// It deregisters all registered instances in parallel and waits for all of them to complete.
func (n *Node) deregisterServiceInstances() {
	eg, ctx := errgroup.WithContext(n.ctx)
	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			tctx, tcancel := context.WithTimeout(ctx, 3*time.Second)
			defer tcancel()
			return n.opts.registry.Deregister(tctx, instance)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Errorf("deregister cluster instances failed: %v", err)
	}
}

// doRegisterServiceInstances performs the registration.
//
// It registers all service instances to the registry in parallel and waits for all of them to
// complete. It reports an error when registering any instance fails.
func (n *Node) doRegisterServiceInstances() error {
	eg, ctx := errgroup.WithContext(n.ctx)

	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			tctx, tcancel := context.WithTimeout(ctx, 3*time.Second)
			err := n.opts.registry.Register(tctx, instance)
			tcancel()

			return err
		})
	}

	return eg.Wait()
}

// doRefreshServiceInstances refreshes the state of the instances.
//
// The optional state is the new state of the service instances; when it is omitted the instances
// are only re-registered without updating the state. It reports an error when registration fails.
func (n *Node) doRefreshServiceInstances(state ...cluster.State) error {
	if len(state) > 0 {
		for _, instance := range n.instances {
			instance.State = state[0].String()
		}
	}

	return n.doRegisterServiceInstances()
}

// getState returns the current node state.
func (n *Node) getState() cluster.State {
	return cluster.State(n.state.Load())
}

// setState updates the state, which may only be switched between Work and Busy.
//
// On success it refreshes the state of the service instances to the registry. It reports an error
// when the state is illegal, the switch fails or refreshing the instances fails.
func (n *Node) setState(state cluster.State) error {
	if state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := n.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if n.state.CompareAndSwap(int32(curr), int32(state)) {
			return n.doRefreshServiceInstances(state)
		} else {
			return errors.ErrIllegalOperation
		}
	default:
		return errors.ErrIllegalOperation
	}
}

// isShut reports whether the node is in the shut state.
func (n *Node) isShut() bool {
	return n.getState() == cluster.Shut
}

// runHookFunc runs the hook functions.
//
// It triggers all listeners of the given hook and waits for all of them to complete.
func (n *Node) runHookFunc(hook cluster.Hook) {
	n.rw.RLock()

	if handlers, ok := n.hooks[hook]; ok {
		wg := &sync.WaitGroup{}
		wg.Add(len(handlers))

		for i := range handlers {
			handler := handlers[i]
			xcall.Go(func() {
				handler(n.proxy)
				wg.Done()
			})
		}

		n.rw.RUnlock()

		wg.Wait()
	} else {
		n.rw.RUnlock()
	}
}

// addHookListener adds a hook listener.
func (n *Node) addHookListener(hook cluster.Hook, handler HookHandler) {
	switch hook {
	case cluster.Destroy:
		n.rw.Lock()
		n.hooks[hook] = append(n.hooks[hook], handler)
		n.rw.Unlock()
	default:
		if n.getState() == cluster.Shut {
			n.rw.Lock()
			n.hooks[hook] = append(n.hooks[hook], handler)
			n.rw.Unlock()
		} else {
			log.Warnf("server is working, can't add hook handler")
		}
	}
}

// addServiceProvider adds a service provider.
func (n *Node) addServiceProvider(name string, desc, provider any) {
	if n.getState() == cluster.Shut {
		n.services = append(n.services, &serviceEntity{
			name:     name,
			desc:     desc,
			provider: provider,
		})
	} else {
		log.Warnf("server is working, can't add service provider")
	}
}

// printInfo prints the component information.
//
// It prints the basic information such as the node ID, name, link address, codec, locator and
// registry.
func (n *Node) printInfo() {
	rows := make([]string, 0, 8)
	rows = append(rows, fmt.Sprintf("ID: %s", n.opts.id))
	rows = append(rows, fmt.Sprintf("Name: %s", n.Name()))
	rows = append(rows, fmt.Sprintf("Link: %s", n.linker.ExposeAddr()))
	rows = append(rows, fmt.Sprintf("Codec: %s", n.opts.codec.Name()))
	rows = append(rows, fmt.Sprintf("Locator: %s", n.opts.locator.Name()))
	rows = append(rows, fmt.Sprintf("Registry: %s", n.opts.registry.Name()))

	if n.opts.encryptor != nil {
		rows = append(rows, fmt.Sprintf("Encryptor: %s", n.opts.encryptor.Name()))
	} else {
		rows = append(rows, "Encryptor: -")
	}

	if n.opts.transporter != nil {
		rows = append(rows, fmt.Sprintf("Transporter: %s", n.opts.transporter.Name()))
	} else {
		rows = append(rows, "Transporter: -")
	}

	info.Print("Node", rows...)
}

// doDoneWait completes one wait count.
//
// It does nothing when the node has been shut; otherwise it decrements the wait count and, when the
// count reaches zero in the hang state, completes the node shutdown.
func (n *Node) doDoneWait() bool {
	if n == nil {
		return false
	}

	state := n.getState()

	if state == cluster.Shut {
		return false
	}

	if n.counter.Add(-1) <= 0 && state == cluster.Hang {
		if n.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
			close(n.done)
		} else {
			return false
		}
	}

	return true
}

// doAddWait adds one wait count.
//
// It only refuses to register in the shut state; registration is still allowed in the hang state so
// that asynchronous tasks submitted from route and event handlers can still be tracked during the
// Close wait. After registering, the state is re-checked and rolled back, avoiding a count that
// nobody waits for when it races with the shutdown decision (Hang to Shut).
func (n *Node) doAddWait() bool {
	if n == nil || n.getState() == cluster.Shut {
		return false
	}

	n.counter.Add(1)

	if n.getState() == cluster.Shut {
		n.counter.Add(-1)

		return false
	}

	return true
}
