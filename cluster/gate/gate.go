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
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xcall"
)

// Gate is the gate server component.
type Gate struct {
	component.Base
	userBindings bindingLocks              // serializes user binding changes
	connBindings bindingLocks              // serializes connection binding changes
	opts         *options                  // gate options
	ctx          context.Context           // component context
	cancel       context.CancelFunc        // cancels the component context
	state        atomic.Int32              // component state
	proxy        *proxy                    // gate proxy
	instance     *registry.ServiceInstance // registered service instance
	registryMu   sync.Mutex                // serializes service instance registration with state refreshes
	session      *session                  // connection and user sessions
	linker       *gate.Server              // internal link server
	limiter      *connLimiter              // connection rate limiter
	security     *securityManager          // session security manager
	mu           sync.Mutex                // serializes connection admission with the transition to Hang or Shut
	wg           sync.WaitGroup            // waits for connection cleanup
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
	g.session = newSession()
	g.state.Store(int32(cluster.Shut))

	if o.security != nil {
		g.security = newSecurityManager(g, o.security, o.securityHandshakeTimeout)
	}

	if o.limiter != nil && o.limiter.normalize() {
		g.limiter = newConnLimiter(o.limiter)
	}

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

	if g.limiter != nil {
		g.limiter.start(g.ctx)
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
	g.mu.Lock()
	if !g.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !g.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			g.mu.Unlock()
			return
		}
	}
	g.mu.Unlock()

	g.refreshServiceInstance()

	g.wg.Wait()
}

// Destroy destroys the component.
//
// It deregisters the service instance, stops the network server and the link server in turn,
// closes the clients to the business nodes, and cancels the component context.
func (g *Gate) Destroy() {
	g.mu.Lock()
	if !g.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()

	g.deregisterServiceInstance()

	g.stopNetworkServer()

	g.wg.Wait()

	g.stopLinkerServer()

	g.proxy.close()

	g.cancel()

	if g.limiter != nil {
		g.limiter.stop()
	}
}

// startNetworkServer starts the network server.
//
// It registers the connect, disconnect and receive handlers and starts the network server,
// logging a fatal error when startup fails. Handshake frames are routed to the security manager
// and bypass the connection rate limiter, so the handshake is never blocked by an exhausted
// business frame budget.
func (g *Gate) startNetworkServer() {
	g.opts.server.OnConnect(g.handleConnect)
	g.opts.server.OnDisconnect(g.handleDisconnect)
	g.opts.server.OnReceive(g.handleReceive)

	if g.security != nil {
		// Handshake frames intentionally bypass the connection rate limiter.
		g.opts.server.OnHandshake(g.security.handleHandshake)
	}

	if err := g.opts.server.Start(); err != nil {
		log.Fatalf("network server start failed: %v", err)
	}
}

// stopNetworkServer stops the network server and shuts down the session security state, logging an
// error when stopping fails.
func (g *Gate) stopNetworkServer() {
	if err := g.opts.server.Stop(); err != nil {
		log.Errorf("network server stop failed: %v", err)
	}

	if g.security != nil {
		g.security.shutdown()
	}
}

// handleConnect handles a newly opened connection.
//
// It accepts the connection and registers the session only when the server is in the Work or Busy
// state; in any other state (Hang or Shut) it closes the connection and rejects it.
func (g *Gate) handleConnect(conn network.Conn) {
	g.mu.Lock()
	if state := cluster.State(g.state.Load()); state == cluster.Work || state == cluster.Busy {
		g.wg.Add(1)

		if g.security != nil {
			g.session.addConn(g.security.wrap(conn))
			g.mu.Unlock()

			// A handshake frame may have arrived before this callback (some transports start the
			// read loop first); process it now that the session has been registered.
			g.security.acceptPending(conn)
		} else {
			g.session.addConn(conn)
			g.mu.Unlock()
			g.proxy.trigger(g.ctx, cluster.Connect, conn.ID(), conn.UID())
		}
	} else {
		g.mu.Unlock()
		if err := conn.Close(); err != nil {
			log.Warnf("close conn failed: %v", err)
		}
	}
}

// handleDisconnect handles a closed connection.
//
// For a connection whose session is registered, it removes the local session synchronously and
// queues the location cleanup and the Disconnect event for asynchronous delivery, completing the
// WaitGroup counter once that work finishes; a connection that is not registered (rejected during
// the closing phase) only has its rate limiter bucket released. When session security is enabled,
// the Disconnect event is triggered only for a connection whose handshake was confirmed, so that
// it stays paired with the Connect event, which is deferred until the handshake is confirmed.
// RemConn atomically performs the existence check and removal, so when concurrent duplicate
// disconnects happen only the first call takes effect, keeping the WaitGroup's Add/Done balanced.
func (g *Gate) handleDisconnect(conn network.Conn) {
	var (
		cid       = conn.ID()
		confirmed = true
	)

	if g.security != nil {
		conn, confirmed = g.security.unwrap(conn), g.security.confirmed(cid)

		g.security.remove(cid)
	}

	// Release the limiter bucket even when the session was never admitted: frames can arrive before
	// the admission or while the connection is closing, so the bucket may have no session owner.
	if g.limiter != nil {
		g.limiter.remove(cid)
	}

	if g.session.remConn(conn) {
		uid := conn.UID()

		// The local session is gone, so the remaining work no longer needs the connection. Queue
		// it so a slow locator does not hold the close callback, and release the wait count after
		// the cleanup and the teardown notification have finished.
		taskpool.Add(func() {
			var userLock *bindingLock

			if uid != 0 {
				// Remove the local connection before waiting for a pending location write. Bind
				// either observes the removal before writing, or this cleanup follows its write.
				userLock = g.userBindings.lock(uid)
				if online, _ := g.session.has(cluster.User, uid); !online {
					ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
					defer cancel()
					_ = g.proxy.cleanupGate(ctx, cid, uid)
				}
			}

			if confirmed {
				g.proxy.trigger(g.ctx, cluster.Disconnect, cid, uid)
			}

			g.wg.Done()

			if userLock != nil {
				userLock.unlock()
			}
		})
	}
}

// handleReceive handles a received business message and delivers the client message to the
// matching business node. Heartbeat frames are handled by the network server before this handler.
// When session security is enabled, handshake frames are routed to the security manager and
// therefore bypass the connection rate limiter.
func (g *Gate) handleReceive(conn network.Conn, buf buffer.Buffer) {
	if g.limiter != nil {
		allowed, closeConn := g.limiter.allow(conn.ID())
		if !allowed {
			buf.Release()

			if closeConn {
				// A transport's Close waits for the connection read loop to exit, and this handler
				// runs inside that loop, so the close must be deferred. The security manager closes
				// connections the same way for the same reason.
				xcall.Go(func() {
					if err := conn.Close(true); err != nil &&
						!errors.Is(err, errors.ErrConnectionClosed) &&
						!errors.Is(err, errors.ErrConnectionNotOpened) {
						log.Warnf("close conn failed: cid = %d, err = %v", conn.ID(), err)
					}
				})
			}

			return
		}
	}

	if g.security != nil && g.security.decrypt(conn, buf) {
		return
	}

	g.proxy.deliver(g.ctx, conn, 0, 0, buf)
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
	instance := &registry.ServiceInstance{
		ID:       g.opts.id,
		Name:     cluster.Gate.String(),
		Kind:     cluster.Gate.String(),
		Alias:    g.opts.name,
		Endpoint: g.linker.Endpoint().String(),
		Metadata: g.opts.metadata,
	}

	g.registryMu.Lock()
	g.instance = instance
	err := g.doRegisterServiceInstance()
	g.registryMu.Unlock()

	if err != nil {
		log.Fatalf("register cluster instance failed: %v", err)
	}
}

// refreshServiceInstance re-registers the service instance with the current state.
func (g *Gate) refreshServiceInstance() {
	if err := g.doRefreshServiceInstance(); err != nil {
		log.Errorf("refresh cluster instance failed: %v", err)
	}
}

// deregisterServiceInstance removes this gate's service instance from the service registry.
//
// It takes the same lock as the refresh path, so an in-flight refresh completes before the
// deregistration and a later refresh cannot register the instance again.
func (g *Gate) deregisterServiceInstance() {
	g.registryMu.Lock()

	// The instance does not exist until Start has registered it, so there is nothing to
	// deregister when the gate is destroyed before the registration completes.
	if g.instance == nil {
		g.registryMu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Deregister(ctx, g.instance)
	cancel()
	g.registryMu.Unlock()

	if err != nil {
		log.Errorf("deregister cluster instance failed: %v", err)
	}
}

// doRegisterServiceInstance registers the service instance with the current state under a timeout.
// The caller must hold registryMu.
func (g *Gate) doRegisterServiceInstance() error {
	g.instance.State = g.getState().String()

	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	err := g.opts.registry.Register(ctx, g.instance)
	cancel()

	return err
}

// doRefreshServiceInstance re-registers the service instance with the current state.
//
// A refresh that arrives before the initial registration has nothing to publish, because Start
// registers the instance with the state at that point. It also skips the registration once the gate
// is shut, so a refresh delayed past Destroy cannot register the instance again after the
// deregistration.
func (g *Gate) doRefreshServiceInstance() error {
	g.registryMu.Lock()
	defer g.registryMu.Unlock()

	if g.isShut() || g.instance == nil {
		return nil
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
	if state < cluster.Work || state > cluster.Busy {
		return errors.ErrIllegalOperation
	}

	switch curr := g.getState(); curr {
	case cluster.Work, cluster.Busy:
		if curr == state {
			return nil
		}

		if g.state.CompareAndSwap(int32(curr), int32(state)) {
			return g.doRefreshServiceInstance()
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

	if g.security != nil {
		rows = append(rows, "Security: enabled")
	} else {
		rows = append(rows, "Security: disabled")
	}

	info.Print("Gate", rows...)
}
