// Package gate implements the gate server component.
package gate

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
)

// Gate is the gate server component.
type Gate struct {
	component.Base
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	state    atomic.Int32
	proxy    *proxy
	instance *registry.ServiceInstance
	session  *session.Session
	linker   *gate.Server
	wg       *sync.WaitGroup
}

// NewGate returns a new gate server component configured by the given options.
func NewGate(opts ...Option) *Gate {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	g := &Gate{}
	g.opts = o
	g.ctx, g.cancel = context.WithCancel(o.ctx)
	g.proxy = newProxy(g)
	g.session = session.NewSession()
	g.state.Store(int32(cluster.Shut))
	g.wg = &sync.WaitGroup{}

	return g
}

// Name returns the component name.
func (g *Gate) Name() string {
	return g.opts.name
}

// Init initializes the component.
//
// It validates whether the instance ID and the required components (server, locator and registry)
// have been injected.
func (g *Gate) Init() {
	if g.opts.id == "" {
		log.Fatal("instance id can not be empty")
	}

	if g.opts.server == nil {
		log.Fatal("server component is not injected")
	}

	if g.opts.locator == nil {
		log.Fatal("locator component is not injected")
	}

	if g.opts.registry == nil {
		log.Fatal("registry component is not injected")
	}
}

// Start starts the component.
//
// It starts the network server and the internal link server in turn, registers the service
// instance, watches user locations and cluster instances, and prints the component information.
func (g *Gate) Start() {
	if !g.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		return
	}

	g.startNetworkServer()

	g.startLinkerServer()

	g.registerServiceInstance()

	g.proxy.watch()

	g.printInfo()
}

// Close closes the gate.
//
// It switches the state to Hang and refreshes the service instance state, then waits for all
// online client sessions to exit naturally (graceful shutdown semantics).
func (g *Gate) Close() {
	if !g.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !g.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	g.refreshServiceInstance()

	g.wg.Wait()
}

// Destroy destroys the component.
//
// It deregisters the service instance, stops the network server and the link server in turn, and
// cancels the component context.
func (g *Gate) Destroy() {
	if !g.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	g.deregisterServiceInstance()

	g.stopNetworkServer()

	g.stopLinkerServer()

	g.cancel()
}

// startNetworkServer starts the network server.
//
// It registers the connect, disconnect and receive handlers and starts the network server,
// logging a fatal error when startup fails.
func (g *Gate) startNetworkServer() {
	g.opts.server.OnConnect(g.handleConnect)
	g.opts.server.OnDisconnect(g.handleDisconnect)
	g.opts.server.OnReceive(g.handleReceive)

	if err := g.opts.server.Start(); err != nil {
		log.Fatalf("network server start failed: %v", err)
	}
}

// stopNetworkServer stops the network server, logging an error when stopping fails.
func (g *Gate) stopNetworkServer() {
	if err := g.opts.server.Stop(); err != nil {
		log.Errorf("network server stop failed: %v", err)
	}
}

// handleConnect handles a newly opened connection.
//
// It accepts the connection and registers the session only when the server is in the Work or Busy
// state; in any other state (Hang or Shut) it closes the connection and rejects it.
func (g *Gate) handleConnect(conn network.Conn) {
	if state := cluster.State(g.state.Load()); state == cluster.Work || state == cluster.Busy {
		g.wg.Add(1)
		g.session.AddConn(conn)
		g.proxy.trigger(g.ctx, cluster.Connect, conn.ID(), conn.UID())
	} else {
		if err := conn.Close(); err != nil {
			log.Warnf("close conn failed: %v", err)
		}
	}
}

// handleDisconnect handles a closed connection.
//
// For a connection whose session is registered, it unbinds the user, triggers the Disconnect event
// and completes the WaitGroup counter; a connection that is not registered (rejected during the
// closing phase) is ignored. RemConn atomically performs the existence check and removal, so when
// concurrent duplicate disconnects happen only the first call takes effect, keeping the WaitGroup's
// Add/Done balanced.
func (g *Gate) handleDisconnect(conn network.Conn) {
	cid := conn.ID()

	if g.session.RemConn(conn) {
		uid := conn.UID()

		if uid != 0 {
			ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
			_ = g.proxy.unbindGate(ctx, cid, uid)
			cancel()
		}

		g.proxy.trigger(g.ctx, cluster.Disconnect, cid, uid)

		g.wg.Done()
	}
}

// handleReceive handles a received message and delivers the client message to the matching
// business node.
func (g *Gate) handleReceive(conn network.Conn, buf buffer.Buffer) {
	g.proxy.deliver(g.ctx, conn, buf)
}

// startLinkerServer starts the transport server.
//
// It creates and asynchronously starts the internal RPC link server.
func (g *Gate) startLinkerServer() {
	linker, err := gate.NewServer(&provider{gate: g}, &gate.ServerOptions{
		Addr:   g.opts.addr,
		Expose: g.opts.expose,
	})
	if err != nil {
		log.Fatalf("linker server create failed: %v", err)
	}

	if err = linker.Start(); err != nil {
		log.Fatalf("linker server start failed: %v", err)
	}

	g.linker = linker
}

// stopLinkerServer stops the internal RPC link server, logging an error when stopping fails.
func (g *Gate) stopLinkerServer() {
	if g.linker == nil {
		return
	}

	if err := g.linker.Stop(); err != nil {
		log.Errorf("linker server stop failed: %v", err)
	}
}

// registerServiceInstance registers the service instance.
//
// It builds the gate service instance information and registers it with the service registry.
func (g *Gate) registerServiceInstance() {
	g.instance = &registry.ServiceInstance{
		ID:       g.opts.id,
		Name:     cluster.Gate.String(),
		Kind:     cluster.Gate.String(),
		Alias:    g.opts.name,
		State:    g.getState().String(),
		Endpoint: g.linker.Endpoint().String(),
		Metadata: g.opts.metadata,
	}

	if err := g.doRegisterServiceInstance(); err != nil {
		log.Fatalf("register cluster instance failed: %v", err)
	}
}

// refreshServiceInstance re-registers the service instance with the current state.
func (g *Gate) refreshServiceInstance() {
	if err := g.doRefreshServiceInstance(g.getState()); err != nil {
		log.Errorf("refresh cluster instance failed: %v", err)
	}
}

// deregisterServiceInstance removes this gate's service instance from the service registry.
func (g *Gate) deregisterServiceInstance() {
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	cancel()
	if err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// doRegisterServiceInstance registers the service instance with the service registry under a
// timeout.
func (g *Gate) doRegisterServiceInstance() error {
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Register(ctx, g.instance)
	cancel()

	return err
}

// doRefreshServiceInstance updates the instance state and re-registers the service instance. The
// optional state is the target state to set.
func (g *Gate) doRefreshServiceInstance(state ...cluster.State) error {
	if len(state) > 0 {
		g.instance.State = state[0].String()
	}

	return g.doRegisterServiceInstance()
}

// getState returns the current state.
func (g *Gate) getState() cluster.State {
	return cluster.State(g.state.Load())
}

// setState updates the state, which may only switch between Work and Busy. On success it refreshes
// the service instance with the new state.
func (g *Gate) setState(state cluster.State) error {
	if state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := g.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if g.state.CompareAndSwap(int32(curr), int32(state)) {
			return g.doRefreshServiceInstance(state)
		} else {
			return errors.ErrIllegalOperation
		}
	default:
		return errors.ErrIllegalOperation
	}
}

// isShut reports whether the current state is Shut.
func (g *Gate) isShut() bool {
	return g.getState() == cluster.Shut
}

// printInfo prints the gate node configuration in an information box.
func (g *Gate) printInfo() {
	rows := make([]string, 0, 6)
	rows = append(rows, fmt.Sprintf("ID: %s", g.opts.id))
	rows = append(rows, fmt.Sprintf("Name: %s", g.Name()))
	rows = append(rows, fmt.Sprintf("Link: %s", g.linker.ExposeAddr()))
	rows = append(rows, fmt.Sprintf("Server: [%s] %s", g.opts.server.Protocol(), net.FulfillAddr(g.opts.server.Addr())))
	rows = append(rows, fmt.Sprintf("Locator: %s", g.opts.locator.Name()))
	rows = append(rows, fmt.Sprintf("Registry: %s", g.opts.registry.Name()))

	info.Print("Gate", rows...)
}
