package client

import (
	"context"

	"github.com/dobyte/due/v2/packet"
)

// Context 路由上下文
// 封装连接与消息信息，供路由处理器使用
type Context struct {
	ctx     context.Context // 上下文
	conn    *Conn           // 连接
	message *packet.Message // 消息
}

// Context 获取上下文
// @return @1 context.Context 上下文
func (c *Context) Context() context.Context {
	return c.ctx
}

// CID 获取连接ID
// @return @1 int64 连接ID
func (c *Context) CID() int64 {
	return c.conn.ID()
}

// UID 获取用户ID
// @return @1 int64 用户ID
func (c *Context) UID() int64 {
	return c.conn.UID()
}

// Conn 获取连接
// @return @1 *Conn 连接对象
func (c *Context) Conn() *Conn {
	return c.conn
}

// Seq 获取消息序列号
// @return @1 int32 消息序列号
func (c *Context) Seq() int32 {
	return c.message.Seq
}

// Route 获取消息路由
// @return @1 int32 消息路由
func (c *Context) Route() int32 {
	return c.message.Route
}

// Data 获取消息数据
// 返回未解密的消息原始内容，需要明文时请使用Parse方法
// @return @1 any 消息数据
func (c *Context) Data() any {
	return c.message.Buffer
}

// Parse 解析消息
// 对消息数据进行解密并反序列化到指定对象
// @param v any 待填充的对象
// @return @1 error 解密或反序列化失败时返回的错误
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
