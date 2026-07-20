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
func (p *Proxy) AddEventHandler(event Event, handler EventHandler) error {
	return p.server.addEventHandler(event, handler)
}

// Client 获取客户端
func (p *Proxy) Client(clientID string) (*Client, bool) {
	if p.server.server == nil {
		return nil, false
	} else {
		if cli, ok := p.server.server.Clients.Get(clientID); ok {
			return &Client{cli: cli}, true
		} else {
			return nil, false
		}
	}
}

// Clients 获取所有客户端
func (p *Proxy) Clients() (map[string]*Client, error) {
	if p.server.server == nil {
		return nil, errors.ErrServerClosed
	} else {
		clis := p.server.server.Clients.GetAll()

		clients := make(map[string]*Client, len(clis))
		for clientID, cli := range clis {
			if clientID == mqtt.InlineClientId {
				continue
			}

			clients[clientID] = &Client{cli: cli}
		}

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
