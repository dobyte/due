package mqtt

import (
	"github.com/dobyte/due/v2/errors"
	mqtt "github.com/mochi-mqtt/server/v2"
)

// Proxy MQTT代理
// 提供MQTT服务器对外可见的完整功能API
type Proxy struct {
	server *Server
}

// SubscribeHandler 内联订阅处理函数
type SubscribeHandler = mqtt.InlineSubFn

// 创建MQTT代理
// @param s *Server MQTT服务器
// @return @1 *Proxy MQTT代理
func newProxy(s *Server) *Proxy {
	return &Proxy{server: s}
}

// AddHook 添加Hook
// 仅在服务启动前可添加，启动后添加返回错误
// @param hook Hook 待添加的Hook
// @param config ...any 可选，Hook配置
// @return @1 error 服务已启动时返回的错误
func (p *Proxy) AddHook(hook Hook, config ...any) error {
	return p.server.addHook(hook, config...)
}

// Client 获取客户端
// @param clientID string 客户端ID
// @return @1 *Client 客户端实例
// @return @2 bool 客户端是否存在
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

// Clients 获取所有客户端
// @return @1 map[string]*Client 客户端ID到客户端实例的映射
// @return @2 error 服务未启动时返回的错误
func (p *Proxy) Clients() (map[string]*Client, error) {
	if p.server.server == nil {
		return nil, errors.ErrServerClosed
	} else {
		return p.server.server.Clients.GetAll(), nil
	}
}

// Publish 发布消息
// @param topic string 主题
// @param payload []byte 消息内容
// @param retain bool 是否保留消息
// @param qos byte 服务质量等级
// @return @1 error 服务未启动或发布失败时返回的错误
func (p *Proxy) Publish(topic string, payload []byte, retain bool, qos byte) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Publish(topic, payload, retain, qos)
	}
}

// Subscribe 订阅主题
// @param filter string 主题过滤器
// @param subscriptionID int 订阅ID
// @param handler SubscribeHandler 订阅处理函数
// @return @1 error 服务未启动或订阅失败时返回的错误
func (p *Proxy) Subscribe(filter string, subscriptionID int, handler SubscribeHandler) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Subscribe(filter, subscriptionID, handler)
	}
}

// Unsubscribe 取消订阅主题
// @param filter string 主题过滤器
// @param subscriptionID int 订阅ID
// @return @1 error 服务未启动或取消订阅失败时返回的错误
func (p *Proxy) Unsubscribe(filter string, subscriptionID int) error {
	if p.server.server == nil {
		return errors.ErrServerClosed
	} else {
		return p.server.server.Unsubscribe(filter, subscriptionID)
	}
}
