package client

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

var (
	ErrClientShut       = errors.New("client is shut")
	ErrConnectionClosed = errors.New("connection closed")
)

type Proxy interface {
	// GetClientID 获取客户端ID
	GetClientID() string
	// AddRouteHandler 添加路由处理器
	AddRouteHandler(route int32, handler RouteHandler)
	// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
	SetDefaultRouteHandler(handler RouteHandler)
	// AddEventListener 添加事件监听器
	AddEventListener(event cluster.Event, handler EventHandler)
	// Bind 绑定用户ID
	Bind(uid int64) error
	// Unbind 解绑用户ID
	Unbind() error
	// Push 推送消息
	Push(seq, route int32, message interface{}) error
	// Reconnect 重新连接
	Reconnect() error
	// Disconnect 断开连接
	Disconnect() error
}

var _ Proxy = &proxy{}

type proxy struct {
	client *Client // 节点
}

func newProxy(client *Client) *proxy {
	return &proxy{client: client}
}

// GetClientID 获取客户端ID
func (p *proxy) GetClientID() string {
	return p.client.opts.id
}

// AddRouteHandler 添加路由处理器
func (p *proxy) AddRouteHandler(route int32, handler RouteHandler) {
	p.client.addRouteHandler(route, handler)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.client.setDefaultRouteHandler(handler)
}

// AddEventListener 添加事件监听器
func (p *proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.client.addEventListener(event, handler)
}

// Bind 绑定用户ID
func (p *proxy) Bind(uid int64) error {
	p.client.rw.RLock()
	defer p.client.rw.RUnlock()

	if p.client.state == cluster.Shut {
		return ErrClientShut
	}

	if p.client.conn == nil {
		return ErrConnectionClosed
	}

	p.client.conn.Bind(uid)

	return nil
}

// Unbind 解绑用户ID
func (p *proxy) Unbind() error {
	p.client.rw.RLock()
	defer p.client.rw.RUnlock()

	if p.client.state == cluster.Shut {
		return ErrClientShut
	}

	if p.client.conn == nil {
		return ErrConnectionClosed
	}

	p.client.conn.Unbind()

	return nil
}

// Push 推送消息
func (p *proxy) Push(seq, route int32, message interface{}) error {
	p.client.rw.RLock()
	defer p.client.rw.RUnlock()

	if p.client.state == cluster.Shut {
		return ErrClientShut
	}

	if p.client.conn == nil {
		return ErrConnectionClosed
	}

	var (
		err    error
		buffer []byte
	)

	if message != nil {
		buffer, err = p.client.opts.codec.Marshal(message)
		if err != nil {
			return err
		}
	}

	if p.client.opts.encryptor != nil {
		buffer, err = p.client.opts.encryptor.Encrypt(buffer)
		if err != nil {
			return err
		}
	}

	msg, err := packet.Pack(&packet.Message{
		Seq:    seq,
		Route:  route,
		Buffer: buffer,
	})
	if err != nil {
		return err
	}

	return p.client.conn.Push(msg)
}

// Reconnect 重新连接
func (p *proxy) Reconnect() error {
	return p.client.dial()
}

// Disconnect 断开连接
func (p *proxy) Disconnect() error {
	p.client.rw.RLock()
	defer p.client.rw.RUnlock()

	if p.client.state == cluster.Shut {
		return ErrClientShut
	}

	if p.client.conn == nil {
		return ErrConnectionClosed
	}

	return p.client.conn.Close()
}
