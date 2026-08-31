package client

import (
	"net"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// Conn 连接封装
// 对底层网络连接进行封装，提供消息推送、属性管理与基础信息获取等能力
type Conn struct {
	conn   network.Conn // 底层网络连接
	client *Client      // 所属客户端
}

// ID 获取连接ID
// @return @1 int64 连接ID
func (c *Conn) ID() int64 {
	return c.conn.ID()
}

// UID 获取用户ID
// @return @1 int64 用户ID
func (c *Conn) UID() int64 {
	return c.conn.UID()
}

// Bind 绑定用户ID
// @param uid int64 用户ID
func (c *Conn) Bind(uid int64) {
	c.conn.Bind(uid)
}

// Unbind 解绑用户ID
func (c *Conn) Unbind() {
	c.conn.Unbind()
}

// SetAttr 设置属性值
// @param key any 属性键
// @param value any 属性值
func (c *Conn) SetAttr(key, value any) {
	c.conn.Attr().Set(key, value)
}

// GetAttr 获取属性值
// @param key any 属性键
// @return @1 value.Value 属性值
func (c *Conn) GetAttr(key any) value.Value {
	if val, ok := c.conn.Attr().Get(key); ok {
		return value.NewValue(val)
	} else {
		return value.NewValue()
	}
}

// DelAttr 删除属性值
// @param key any 属性键
func (c *Conn) DelAttr(key any) {
	c.conn.Attr().Del(key)
}

// LocalIP 获取本地IP
// @return @1 string 本地IP地址
// @return @2 error 错误信息
func (c *Conn) LocalIP() (string, error) {
	return c.conn.LocalIP()
}

// LocalAddr 获取本地地址
// @return @1 net.Addr 本地地址
// @return @2 error 错误信息
func (c *Conn) LocalAddr() (net.Addr, error) {
	return c.conn.LocalAddr()
}

// RemoteIP 获取远端IP
// @return @1 string 远端IP地址
// @return @2 error 错误信息
func (c *Conn) RemoteIP() (string, error) {
	return c.conn.RemoteIP()
}

// RemoteAddr 获取远端地址
// @return @1 net.Addr 远端地址
// @return @2 error 错误信息
func (c *Conn) RemoteAddr() (net.Addr, error) {
	return c.conn.RemoteAddr()
}

// Push 推送消息
// 对消息数据进行编解码与加密处理后推送给服务端
// @param message *cluster.Message 待推送的消息
// @return @1 error 错误信息
func (c *Conn) Push(message *cluster.Message) error {
	var (
		err    error
		buffer []byte
	)

	if message.Data != nil {
		if v, ok := message.Data.([]byte); ok {
			buffer = v
		} else {
			if buffer, err = c.client.opts.codec.Marshal(message.Data); err != nil {
				return err
			}
		}

		if c.client.opts.encryptor != nil {
			if buffer, err = c.client.opts.encryptor.Encrypt(buffer); err != nil {
				return err
			}
		}
	}

	msg, err := packet.PackMessage(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buffer,
	})
	if err != nil {
		return err
	}

	return c.conn.Push(msg)
}

// Close 关闭连接
// @return @1 error 错误信息
func (c *Conn) Close() error {
	return c.conn.Close()
}
