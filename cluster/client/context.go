package client

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/packet"
)

type Context struct {
	ctx     context.Context // 上下文
	client  *Client         // 客户端
	message *packet.Message // 消息
}

// Proxy 响应请求
func (c *Context) Proxy() *Proxy {
	return c.client.proxy
}

// Context 获取上线文
func (c *Context) Context() context.Context {
	return c.ctx
}

// CID 获取连接ID
func (c *Context) CID() int64 {
	return c.client.conn.ID()
}

// UID 获取用户ID
func (c *Context) UID() int64 {
	return c.client.conn.UID()
}

// Seq 获取消息序列号
func (c *Context) Seq() int32 {
	return c.message.Seq
}

// Route 获取消息路由
func (c *Context) Route() int32 {
	return c.message.Route
}

// Data 获取消息数据
func (c *Context) Data() interface{} {
	return c.message.Buffer
}

// Parse 解析消息
func (c *Context) Parse(v interface{}) (err error) {
	buffer := c.message.Buffer

	if c.client.opts.encryptor != nil {
		buffer, err = c.client.opts.encryptor.Decrypt(buffer)
		if err != nil {
			return
		}
	}

	return c.client.opts.codec.Unmarshal(buffer, v)
}

// Bind 绑定用户ID
func (c *Context) Bind(uid int64) error {
	return c.client.proxy.Bind(uid)
}

// Unbind 解绑用户ID
func (c *Context) Unbind() error {
	return c.client.proxy.Unbind()
}

// Push 推送消息
func (c *Context) Push(message *cluster.Message) error {
	return c.client.proxy.Push(message)
}
