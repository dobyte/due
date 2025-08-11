package client

import (
	"context"

	"github.com/dobyte/due/v2/packet"
)

type Context struct {
	ctx     context.Context // 上下文
	conn    *Conn           // 连接
	message *packet.Message // 消息
}

// Context 获取上线文
func (c *Context) Context() context.Context {
	return c.ctx
}

// CID 获取连接ID
func (c *Context) CID() int64 {
	return c.conn.ID()
}

// UID 获取用户ID
func (c *Context) UID() int64 {
	return c.conn.UID()
}

// Conn 获取连接
func (c *Context) Conn() *Conn {
	return c.conn
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
func (c *Context) Data() any {
	return c.message.Buffer
}

// Parse 解析消息
func (c *Context) Parse(v any) (err error) {
	buffer := c.message.Buffer

	if c.conn.client.opts.encryptor != nil {
		buffer, err = c.conn.client.opts.encryptor.Decrypt(buffer)
		if err != nil {
			return
		}
	}

	return c.conn.client.opts.codec.Unmarshal(buffer, v)
}
