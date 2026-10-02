package mqtt

import (
	"github.com/dobyte/due/v2/errors"
	mqtt "github.com/mochi-mqtt/server/v2"
)

// Proxy is the MQTT proxy.
//
// It exposes the full set of features that an MQTT [Server] provides to the outside.
type Proxy struct {
	server *Server
}

// SubscribeHandler is an inline subscription handler.
type SubscribeHandler = mqtt.InlineSubFn

// newProxy creates an MQTT proxy.
func newProxy(s *Server) *Proxy {
	return &Proxy{server: s}
}

// AddHook adds a hook.
//
// A hook can only be added before the server starts; adding one after the server has started
// returns an error. hook is the hook to add and the optional config is the hook configuration.
func (p *Proxy) AddHook(hook Hook, config ...any) error {
	return p.server.addHook(hook, config...)
}

// Client returns the client identified by clientID and reports whether it exists.
func (p *Proxy) Client(clientID string) (*Client, bool) {
	if p.server.server == nil {
		return nil, false
	} else {
		if cli, ok := p.server.server.Clients.Get(clientID); ok {
			return cli, true
		} else {
			return nil, false
		}
	}
}

// Clients returns a map from client ID to client instance. It returns an error when the server
// has not started.
func (p *Proxy) Clients() (map[string]*Client, error) {
	if p.server.server == nil {
		return nil, errors.ErrServerClosed
	} else {
		return p.server.server.Clients.GetAll(), nil
	}
}

// Publish publishes payload to topic.
//
// retain reports whether the message is retained and qos is the quality of service level. It
// returns an error when the server has not started or the publish fails.
func (p *Proxy) Publish(topic string, payload []byte, retain bool, qos byte) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Publish(topic, payload, retain, qos)
	}
}

// Subscribe subscribes to filter.
//
// subscriptionID is the subscription ID and handler is the subscription handler. It returns an
// error when the server has not started or the subscription fails.
func (p *Proxy) Subscribe(filter string, subscriptionID int, handler SubscribeHandler) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Subscribe(filter, subscriptionID, handler)
	}
}

// Unsubscribe unsubscribes from filter.
//
// subscriptionID is the subscription ID. It returns an error when the server has not started or
// the unsubscribe fails.
func (p *Proxy) Unsubscribe(filter string, subscriptionID int) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Unsubscribe(filter, subscriptionID)
	}
}
