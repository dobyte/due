package client

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xcall"
)

// HookHandler is the hook handler of the client.
type HookHandler func(proxy *Proxy)

// RouteHandler is the route handler of the client.
type RouteHandler func(ctx *Context)

// EventHandler is the event handler of the client.
type EventHandler func(conn *Conn)

// Client is the client component.
//
// It dials connections to the server and provides registration of routes, events and hooks as
// well as message sending and receiving.
type Client struct {
	component.Base
	opts                *options           // options
	ctx                 context.Context    // context
	cancel              context.CancelFunc // cancel function
	proxy               *Proxy             // client proxy
	state               atomic.Int32       // client state
	rw1                 sync.Mutex         // registration lock, guards concurrent registration of routes, events and hooks
	hooks               atomic.Value       // hook handlers (map[cluster.Hook][]HookHandler)
	routes              atomic.Value       // route handlers (map[int32][]RouteHandler)
	events              atomic.Value       // event handlers (map[cluster.Event][]EventHandler)
	defaultRouteHandler atomic.Value       // default route handler (RouteHandler)
	rw2                 sync.RWMutex       // connection lock, guards concurrent access to the connection table
	conns               sync.Map           // connection table (network.Conn -> *Conn)
}

// NewClient returns a new client component configured by the given options.
func NewClient(opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &Client{}
	c.opts = o
	c.proxy = newProxy(c)
	c.hooks.Store(make(map[cluster.Hook][]HookHandler))
	c.routes.Store(make(map[int32][]RouteHandler))
	c.events.Store(make(map[cluster.Event][]EventHandler))
	c.defaultRouteHandler.Store(RouteHandler(nil))
	c.ctx, c.cancel = context.WithCancel(o.ctx)
	c.state.Store(int32(cluster.Shut))

	return c
}

// Name returns the client name.
func (c *Client) Name() string {
	return c.opts.name
}

// Init initializes the client.
//
// It validates required configuration such as the network client and codec, terminating the
// process when they are missing, and then triggers the Init hook.
func (c *Client) Init() {
	if c.opts.client == nil {
		log.Fatal("client plugin is not injected")
	}

	if c.opts.codec == nil {
		log.Fatal("codec plugin is not injected")
	}

	c.runHookFunc(cluster.Init)
}

// Start starts the component.
func (c *Client) Start() {
	c.rw1.Lock()

	if !c.state.CompareAndSwap(int32(cluster.Shut), int32(cluster.Work)) {
		c.rw1.Unlock()
		return
	}

	c.opts.client.OnDisconnect(c.handleDisconnect)
	c.opts.client.OnReceive(c.handleReceive)

	c.rw1.Unlock()

	c.printInfo()

	c.runHookFunc(cluster.Start)
}

// Close closes the client.
func (c *Client) Close() {
	if !c.state.CompareAndSwap(int32(cluster.Work), int32(cluster.Hang)) {
		if !c.state.CompareAndSwap(int32(cluster.Busy), int32(cluster.Hang)) {
			return
		}
	}

	c.runHookFunc(cluster.Close)
}

// Destroy destroys the client.
//
// It marks the client as shut, closes every connection and triggers the Destroy hook.
func (c *Client) Destroy() {
	if !c.state.CompareAndSwap(int32(cluster.Hang), int32(cluster.Shut)) {
		return
	}

	c.cancel()

	conns := make([]*Conn, 0)

	c.rw2.Lock()
	c.conns.Range(func(_, conn any) bool {
		conns = append(conns, conn.(*Conn))
		return true
	})
	c.conns.Clear()
	c.rw2.Unlock()

	for _, cc := range conns {
		cc.Close()
	}

	c.runHookFunc(cluster.Destroy)
}

// Proxy returns the client proxy.
func (c *Client) Proxy() *Proxy {
	return c.proxy
}

// handleDisconnect handles a closed connection. It removes the connection from the connection
// table and triggers the disconnect event.
func (c *Client) handleDisconnect(conn network.Conn) {
	val, ok := c.conns.Load(conn)
	if !ok {
		return
	}

	c.conns.Delete(conn)

	if handlers, ok := c.events.Load().(map[cluster.Event][]EventHandler)[cluster.Disconnect]; ok {
		for _, handler := range handlers {
			xcall.Call(func() {
				handler(val.(*Conn))
			})
		}
	}
}

// handleReceive handles a received message. It unpacks the message and dispatches it to the
// matching route handler, falling back to the default route handler when the route is not
// registered.
func (c *Client) handleReceive(conn network.Conn, buf buffer.Buffer) {
	val, ok := c.conns.Load(conn)
	if !ok {
		buf.Release()
		return
	}

	route, seq, data, err := packet.UnpackMessage(buf)
	if err != nil {
		buf.Release()
		log.Errorf("unpack message failed: %v", err)
		return
	}

	if handlers, ok := c.routes.Load().(map[int32][]RouteHandler)[route]; ok {
		for _, handler := range handlers {
			xcall.Call(func() {
				handler(&Context{
					ctx:   context.Background(),
					conn:  val.(*Conn),
					route: route,
					seq:   seq,
					buf:   data,
				})
			})
		}
	} else if handler := c.defaultRouteHandler.Load().(RouteHandler); handler != nil {
		xcall.Call(func() {
			handler(&Context{
				ctx:   context.Background(),
				conn:  val.(*Conn),
				route: route,
				seq:   seq,
				buf:   data,
			})
		})
	} else {
		log.Debugf("route handler is not registered, route: %v", route)
	}

	buf.Release()
}

// dial dials a connection to the server.
func (c *Client) dial(opts ...DialOption) (*Conn, error) {
	if st := c.getState(); st != cluster.Work && st != cluster.Busy {
		return nil, errors.ErrClientShut
	}

	o := &dialOptions{attrs: make(map[string]any)}
	for _, opt := range opts {
		opt(o)
	}

	conn, err := c.opts.client.Dial(o.addr)
	if err != nil {
		return nil, err
	}

	cc := &Conn{conn: conn, client: c}

	for key, value := range o.attrs {
		cc.SetAttr(key, value)
	}

	c.rw2.Lock()
	if st := c.getState(); st != cluster.Work && st != cluster.Busy {
		c.rw2.Unlock()
		conn.Close()
		return nil, errors.ErrClientShut
	}
	c.conns.Store(conn, cc)
	c.rw2.Unlock()

	if handlers, ok := c.events.Load().(map[cluster.Event][]EventHandler)[cluster.Connect]; ok {
		for _, handler := range handlers {
			xcall.Call(func() {
				handler(cc)
			})
		}
	}

	return cc, nil
}

// addEventListener adds an event handler for the given event.
func (c *Client) addEventListener(event cluster.Event, handler EventHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if c.getState() != cluster.Shut {
		log.Warnf("client is working, can't add event handler")
		return
	}

	oldEvents := c.events.Load().(map[cluster.Event][]EventHandler)
	newEvents := maps.Clone(oldEvents)
	newEvents[event] = append(newEvents[event], handler)

	c.events.Store(newEvents)
}

// addRouteHandler adds a route handler for the given route.
func (c *Client) addRouteHandler(route int32, handler RouteHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if c.getState() != cluster.Shut {
		log.Warnf("client is working, can't add route handler")
		return
	}

	oldRoutes := c.routes.Load().(map[int32][]RouteHandler)
	newRoutes := maps.Clone(oldRoutes)
	newRoutes[route] = append(newRoutes[route], handler)

	c.routes.Store(newRoutes)
}

// setDefaultRouteHandler sets the default route handler that serves every unregistered route. It
// may be set only once.
func (c *Client) setDefaultRouteHandler(handler RouteHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if c.getState() != cluster.Shut {
		log.Warnf("client is working, can't set default route handler")
		return
	}

	if cur := c.defaultRouteHandler.Load().(RouteHandler); cur != nil {
		log.Warnf("default route handler is already set")
		return
	}

	c.defaultRouteHandler.Store(handler)
}

// addHookListener adds a hook listener for the given hook.
func (c *Client) addHookListener(hook cluster.Hook, handler HookHandler) {
	c.rw1.Lock()
	defer c.rw1.Unlock()

	if hook != cluster.Destroy && c.getState() != cluster.Shut {
		log.Warnf("client is working, can't add hook handler")
		return
	}

	oldHooks := c.hooks.Load().(map[cluster.Hook][]HookHandler)
	newHooks := maps.Clone(oldHooks)
	newHooks[hook] = append(newHooks[hook], handler)

	c.hooks.Store(newHooks)
}

// getState returns the client state.
func (c *Client) getState() cluster.State {
	return cluster.State(c.state.Load())
}

// runHookFunc runs every listener of the given hook and waits for all of them to finish.
func (c *Client) runHookFunc(hook cluster.Hook) {
	handlers, ok := c.hooks.Load().(map[cluster.Hook][]HookHandler)[hook]
	if !ok {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(handlers))

	for i := range handlers {
		handler := handlers[i]
		xcall.Go(func() {
			handler(c.proxy)
			wg.Done()
		})
	}

	wg.Wait()
}

// printInfo prints basic component information such as the client name, codec, protocol and
// encryptor.
func (c *Client) printInfo() {
	rows := make([]string, 0, 4)
	rows = append(rows, fmt.Sprintf("Name: %s", c.Name()))
	rows = append(rows, fmt.Sprintf("Codec: %s", c.opts.codec.Name()))
	rows = append(rows, fmt.Sprintf("Protocol: %s", c.opts.client.Protocol()))

	if c.opts.encryptor != nil {
		rows = append(rows, fmt.Sprintf("Encryptor: %s", c.opts.encryptor.Name()))
	} else {
		rows = append(rows, "Encryptor: -")
	}

	info.Print("Client", rows...)
}
