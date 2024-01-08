package client

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
)

type Proxy struct {
	client *Client // 客户端
}

func newProxy(client *Client) *Proxy {
	return &Proxy{client: client}
}

// GetClientID 获取客户端ID
func (p *Proxy) GetClientID() string {
	return p.client.opts.id
}

// AddRouteHandler 添加路由处理器
func (p *Proxy) AddRouteHandler(route int32, handler RouteHandler) {
	p.client.addRouteHandler(route, handler)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.client.setDefaultRouteHandler(handler)
}

// AddEventListener 添加事件监听器
func (p *Proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.client.addEventListener(event, handler)
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.client.addHookListener(hook, handler)
}

// Dial 拨号
func (p *Proxy) Dial(addr ...string) (*Conn, error) {
	return p.client.dial(addr...)
}

// Bind 绑定用户ID
func (p *Proxy) Bind(uid int64) error {
	p.client.rw.RLock()
	defer p.client.rw.RUnlock()

	if p.client.state == cluster.Shut {
		return errors.ErrClientShut
	}

	if p.client.conn == nil {
		return errors.ErrConnectionClosed
	}

	p.client.conn.Bind(uid)

	return nil
}

// Unbind 解绑用户ID
func (p *Proxy) Unbind() error {
	p.client.rw.RLock()
	defer p.client.rw.RUnlock()

	if p.client.state == cluster.Shut {
		return errors.ErrClientShut
	}

	if p.client.conn == nil {
		return errors.ErrConnectionClosed
	}

	p.client.conn.Unbind()

	return nil
}

//// Push 推送消息
//func (p *Proxy) Push(message *cluster.Message) error {
//	p.client.rw.RLock()
//	defer p.client.rw.RUnlock()
//
//	if p.client.state == cluster.Shut {
//		return errors.ErrClientShut
//	}
//
//	if p.client.conn == nil {
//		return errors.ErrConnectionClosed
//	}
//
//	var (
//		err    error
//		buffer []byte
//	)
//
//	if v, ok := message.Data.([]byte); ok {
//		buffer = v
//	} else {
//		buffer, err = p.client.opts.codec.Marshal(message.Data)
//		if err != nil {
//			return err
//		}
//	}
//
//	if p.client.opts.encryptor != nil {
//		buffer, err = p.client.opts.encryptor.Encrypt(buffer)
//		if err != nil {
//			return err
//		}
//	}
//
//	msg, err := packet.Pack(&packet.Message{
//		Seq:    message.Seq,
//		Route:  message.Route,
//		Buffer: buffer,
//	})
//	if err != nil {
//		return err
//	}
//
//	return p.client.conn.Push(msg)
//}
//
//// Disconnect 断开连接
//func (p *Proxy) Disconnect() error {
//	p.client.rw.RLock()
//	defer p.client.rw.RUnlock()
//
//	if p.client.state == cluster.Shut {
//		return errors.ErrClientShut
//	}
//
//	if p.client.conn == nil {
//		return errors.ErrConnectionClosed
//	}
//
//	return p.client.conn.Close()
//}
