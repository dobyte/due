package client

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"net"
	"sync"
)

type Conn struct {
	conn       network.Conn
	client     *Client
	attributes sync.Map
}

// ID 获取连接ID
func (c *Conn) ID() int64 {
	return c.conn.ID()
}

// UID 获取用户ID
func (c *Conn) UID() int64 {
	return c.conn.UID()
}

// Bind 绑定用户ID
func (c *Conn) Bind(uid int64) {
	c.conn.Bind(uid)
}

// Unbind 解绑用户ID
func (c *Conn) Unbind() {
	c.conn.Unbind()
}

// SetAttributes 设置属性值
func (c *Conn) SetAttributes(key, value any) {
	c.attributes.Store(key, value)
}

// GetAttributes 获取属性值
func (c *Conn) GetAttributes(key any) (any, bool) {
	return c.attributes.Load(key)
}

// DelAttributes 删除属性值
func (c *Conn) DelAttributes(key any) {
	c.attributes.Delete(key)
}

// LocalIP 获取本地IP
func (c *Conn) LocalIP() (string, error) {
	return c.conn.LocalIP()
}

// LocalAddr 获取本地地址
func (c *Conn) LocalAddr() (net.Addr, error) {
	return c.conn.LocalAddr()
}

// RemoteIP 获取远端IP
func (c *Conn) RemoteIP() (string, error) {
	return c.conn.RemoteIP()
}

// RemoteAddr 获取远端地址
func (c *Conn) RemoteAddr() (net.Addr, error) {
	return c.conn.RemoteAddr()
}

// Push 推送消息
func (c *Conn) Push(message *cluster.Message) error {
	var (
		err    error
		buffer []byte
	)

	if v, ok := message.Data.([]byte); ok {
		buffer = v
	} else {
		buffer, err = c.client.opts.codec.Marshal(message.Data)
		if err != nil {
			return err
		}
	}

	if c.client.opts.encryptor != nil {
		buffer, err = c.client.opts.encryptor.Encrypt(buffer)
		if err != nil {
			return err
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
func (c *Conn) Close() error {
	return c.conn.Close()
}
