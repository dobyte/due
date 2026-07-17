package mqtt

import (
	"net"

	"github.com/dobyte/due/v2/utils/xnet"
	mqtt "github.com/mochi-mqtt/server/v2"
)

type Client struct {
	cli *mqtt.Client
}

// ID 获取客户端ID
func (c *Client) ID() string {
	return c.cli.ID
}

// Listener 获取客户端连接监听器ID
func (c *Client) Listener() string {
	return c.cli.Net.Listener
}

// Topic 获取客户端订阅的主题
func (c *Client) Topic() string {
	return c.cli.Properties.Will.TopicName
}

// LocalIP 获取本地IP
func (c *Client) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
func (c *Client) LocalAddr() (net.Addr, error) {
	return c.cli.Net.Conn.LocalAddr(), nil
}

// RemoteIP 获取远端IP
func (c *Client) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
func (c *Client) RemoteAddr() (net.Addr, error) {
	return c.cli.Net.Conn.RemoteAddr(), nil
}

// Username 获取客户端用户名
func (c *Client) Username() string {
	return string(c.cli.Properties.Username)
}

// ProtocolVersion 获取客户端协议版本
func (c *Client) ProtocolVersion() byte {
	return c.cli.Properties.ProtocolVersion
}
