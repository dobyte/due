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

// AddEventHandler 添加事件处理器
func (p *Proxy) AddEventHandler(event Event, handler EventHandler) {
	p.server.addEventHandler(event, handler)
}

// Client 获取客户端
func (p *Proxy) Client(clientID string) (*mqtt.Client, bool) {
	if p.server.server == nil {
		return nil, false
	} else {
		return p.server.server.Clients.Get(clientID)
	}
}

// Clients 获取所有客户端
func (p *Proxy) Clients() (map[string]*mqtt.Client, error) {
	if p.server.server == nil {
		return nil, errors.ErrServerClosed
	} else {
		clients := p.server.server.Clients.GetAll()

		delete(clients, mqtt.InlineClientId)

		return clients, nil
	}
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
func (p *Proxy) Subscribe(filter string, subscriptionID int, handler SubscribeHandler) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Subscribe(filter, subscriptionID, handler)
	}
}

// Unsubscribe 取消订阅主题
func (p *Proxy) Unsubscribe(filter string, subscriptionID int) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Unsubscribe(filter, subscriptionID)
	}
}
