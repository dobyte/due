package node

import (
	"context"
	"github.com/dobyte/due/session"
)

type Context struct {
	ctx        context.Context
	Proxy      *Proxy
	Request    *Request
	Middleware *Middleware
}

// Context 获取上线文
func (c *Context) Context() context.Context {
	return c.ctx
}

// BindGate 绑定网关
func (c *Context) BindGate(ctx context.Context, uid int64) error {
	return c.Proxy.BindGate(ctx, c.Request.gid, c.Request.cid, uid)
}

// UnbindGate 解绑网关
func (c *Context) UnbindGate(ctx context.Context) error {
	return c.Proxy.UnbindGate(ctx, c.Request.uid)
}

// BindNode 绑定节点
func (c *Context) BindNode() error {
	return c.Proxy.BindNode(c.ctx, c.Request.uid)
}

// UnbindNode 解绑节点
func (c *Context) UnbindNode() error {
	return c.Proxy.UnbindNode(c.ctx, c.Request.uid)
}

// GetIP 获取来自网关的连接IP地址
func (c *Context) GetIP() (string, error) {
	return c.Proxy.GetIP(c.ctx, &GetIPArgs{
		GID:    c.Request.gid,
		Kind:   session.Conn,
		Target: c.Request.cid,
	})
}

// Response 对来自网关或节点的请求进行响应（C/S模型）
// 使用此方法响应的（消息序列号和消息路由号）与请求的（消息序列号和消息路由号）保持一致
func (c *Context) Response(message interface{}) error {
	return c.Proxy.Response(c.ctx, c.Request, message)
}

// Disconnect 关闭来自网关的连接
func (c *Context) Disconnect(isForce ...bool) error {
	args := &DisconnectArgs{
		GID:    c.Request.gid,
		Kind:   session.Conn,
		Target: c.Request.cid,
	}

	if len(isForce) > 0 {
		args.IsForce = isForce[0]
	}

	return c.Proxy.Disconnect(c.ctx, args)
}
