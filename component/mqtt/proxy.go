package mqtt

import (
	"github.com/dobyte/due/v2/errors"
	mqtt "github.com/mochi-mqtt/server/v2"
)

type Proxy struct {
	server *Server
}

type SubscribeHandler = mqtt.InlineSubFn

func newProxy(s *Server) *Proxy {
	return &Proxy{server: s}
}

// Publish 发布消息
func (p *Proxy) Publish(topic string, payload []byte, retain bool, qos byte) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Publish(topic, payload, retain, qos)
	}
}

// Subscribe 订阅主题
func (p *Proxy) Subscribe(filter string, subscriptionId int, handler SubscribeHandler) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Subscribe(filter, subscriptionId, handler)
	}
}
