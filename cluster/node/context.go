package node

import (
	"context"
	"github.com/dobyte/due/session"
)

type Context struct {
	ctx        context.Context
	Proxy      *Proxy
	Request    *Request    // 请求
	Middleware *Middleware // 中间件
}

// Context 获取上线文
func (c *Context) Context() context.Context {
	return c.ctx
}

// GetIP 获取IP地址
func (c *Context) GetIP() (string, error) {
	return c.Proxy.GetIP(c.ctx, &GetIPArgs{
		GID:    c.Request.gid,
		Kind:   session.Conn,
		Target: c.Request.cid,
	})
}

// Response 响应请求
func (c *Context) Response(message interface{}) error {
	return c.Proxy.Response(c.ctx, c.Request, message)
}

// BindGate 绑定网关
func (c *Context) BindGate(uid int64) error {
	return c.Proxy.BindGate(c.ctx, c.Request.gid, c.Request.cid, uid)
}

// UnbindGate 解绑网关
func (c *Context) UnbindGate() error {
	return c.Proxy.UnbindGate(c.ctx, c.Request.uid)
}

// BindNode 绑定节点
func (c *Context) BindNode() error {
	return c.Proxy.BindNode(c.ctx, c.Request.uid)
}

// UnbindNode 解绑节点
func (c *Context) UnbindNode() error {
	return c.Proxy.UnbindNode(c.ctx, c.Request.uid)
}

// Disconnect 断开连接
func (c *Context) Disconnect(isForce ...bool) error {
	if c.Request.gid == "" {
		return nil
	}

	isForceClose := false
	if len(isForce) > 0 && isForce[0] {
		isForceClose = true
	}

	return c.Proxy.Disconnect(c.ctx, &DisconnectArgs{
		GID:     c.Request.gid,
		Kind:    session.Conn,
		Target:  c.Request.cid,
		IsForce: isForceClose,
	})
}
