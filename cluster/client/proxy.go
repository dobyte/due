package client

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/network"
)

// Proxy is the client proxy.
//
// It exposes the registration of routes, events and hooks as well as dialing.
type Proxy struct {
	client *Client // client
}

// newProxy returns a new client proxy for the given client.
func newProxy(client *Client) *Proxy {
	return &Proxy{client: client}
}

// ID returns the client ID.
func (p *Proxy) ID() string {
	return p.client.opts.id
}

// Name returns the client name.
func (p *Proxy) Name() string {
	return p.client.opts.name
}

// AddRouteHandler adds a route handler for the given route.
func (p *Proxy) AddRouteHandler(route int32, handler RouteHandler) {
	p.client.addRouteHandler(route, handler)
}

// SetDefaultRouteHandler sets the default route handler that serves every unregistered route.
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.client.setDefaultRouteHandler(handler)
}

// AddEventListener adds an event listener for the given event.
func (p *Proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.client.addEventListener(event, handler)
}

// AddHookListener adds a hook listener for the given hook.
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.client.addHookListener(hook, handler)
}

// Dial dials a connection to the server and returns the wrapped connection.
func (p *Proxy) Dial(opts ...DialOption) (*Conn, error) {
	return p.client.dial(opts...)
}

// Client returns the network client.
func (p *Proxy) Client() network.Client {
	return p.client.opts.client
}
