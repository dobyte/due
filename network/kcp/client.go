package kcp

import (
	"github.com/symsimmy/due/network"
	"github.com/xtaci/kcp-go"
)

type client struct {
	opts              *clientOptions            // 配置
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Client = &client{}

func NewClient(opts ...ClientOption) network.Client {
	o := &clientOptions{
		addr:         "127.0.0.1:3553",
		maxMsgLength: 1024 * 1024,
	}
	for _, opt := range opts {
		opt(o)
	}

	return &client{opts: o}
}

// Dial 拨号连接
func (c *client) Dial() (network.Conn, error) {
	conn, err := kcp.Dial(c.opts.addr)
	if err != nil {
		return nil, err
	}

	return newClientConn(c, conn), nil
}

// OnConnect 监听连接打开
func (c *client) OnConnect(handler network.ConnectHandler) {
	c.connectHandler = handler
}

// OnDisconnect 监听连接关闭
func (c *client) OnDisconnect(handler network.DisconnectHandler) {
	c.disconnectHandler = handler
}

// OnReceive 监听接收到消息
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}
