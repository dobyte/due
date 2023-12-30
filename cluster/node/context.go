package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/session"
)

type Context struct {
	ctx        context.Context
	Proxy      *Proxy
	Request    *Request
	Middleware *Middleware
}

// Clone 克隆Context
func (c *Context) Clone() *Context {
	return &Context{
		ctx:   context.Background(),
		Proxy: c.Proxy,
		Request: &Request{
			node: c.Request.node,
			GID:  c.Request.GID,
			NID:  c.Request.NID,
			CID:  c.Request.CID,
			UID:  c.Request.UID,
			Message: &cluster.Message{
				Seq:   c.Request.Message.Seq,
				Route: c.Request.Message.Route,
				Data:  c.Request.Message.Data,
			},
		},
		Middleware: &Middleware{
			index:       c.Middleware.index,
			middlewares: c.Middleware.middlewares,
		},
	}
}

// Context 获取上下文
func (c *Context) Context() context.Context {
	return c.ctx
}

// BindGate 绑定网关
func (c *Context) BindGate(uid int64) error {
	return c.Proxy.BindGate(c.ctx, uid, c.Request.GID, c.Request.CID)
}

// UnbindGate 解绑网关
func (c *Context) UnbindGate() error {
	return c.Proxy.UnbindGate(c.ctx, c.Request.UID)
}

// BindNode 绑定节点
func (c *Context) BindNode() error {
	return c.Proxy.BindNode(c.ctx, c.Request.UID)
}

// UnbindNode 解绑节点
func (c *Context) UnbindNode() error {
	return c.Proxy.UnbindNode(c.ctx, c.Request.UID)
}

// GetIP 获取来自网关的连接IP地址
func (c *Context) GetIP() (string, error) {
	return c.Proxy.GetIP(c.ctx, &cluster.GetIPArgs{
		GID:    c.Request.GID,
		Kind:   session.Conn,
		Target: c.Request.CID,
	})
}

// Response 对来自网关或节点的请求进行响应（C/S模型）
// 使用此方法响应的（消息序列号和消息路由号）与请求的（消息序列号和消息路由号）保持一致
func (c *Context) Response(message interface{}) error {
	return c.Proxy.Response(c.ctx, c.Request, message)
}

// Disconnect 关闭来自网关的连接
func (c *Context) Disconnect(isForce ...bool) error {
	args := &link.DisconnectArgs{
		GID:    c.Request.GID,
		Kind:   session.Conn,
		Target: c.Request.CID,
	}

	if len(isForce) > 0 {
		args.IsForce = isForce[0]
	}

	return c.Proxy.Disconnect(c.ctx, args)
}
