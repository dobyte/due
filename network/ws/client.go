package ws

import (
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/gorilla/websocket"
)

type client struct {
	opts              *clientOptions            // 配置
	id                atomic.Int64              // 连接ID
	dialer            *websocket.Dialer         // 拨号器
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	heartbeatHandler  network.HeartbeatHandler  // 连接心跳hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Client = &client{}

// NewClient 创建一个客户端
// @param opts ...ClientOption 客户端配置项
// @return @1 network.Client 客户端实例
func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &client{}
	c.opts = o
	c.dialer = &websocket.Dialer{
		HandshakeTimeout:  o.dialTimeout,
		EnableCompression: o.compression,
	}

	return c
}

// Dial 拨号连接
// @param addr ...string 拨号地址
// @return @1 network.Conn 连接对象
// @return @2 error 错误信息
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var url string

	if len(addr) > 0 && addr[0] != "" {
		url = addr[0]
	} else {
		url = c.opts.url
	}

	conn, _, err := c.dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return newClientConn(c.id.Add(1), conn, c), nil
}

// Protocol 获取协议名称
// @return @1 string 协议名称
func (c *client) Protocol() string {
	return protocol
}

// OnConnect 监听连接打开
// @param handler network.ConnectHandler 连接打开处理函数
func (c *client) OnConnect(handler network.ConnectHandler) {
	c.connectHandler = handler
}

// OnDisconnect 监听连接关闭
// @param handler network.DisconnectHandler 连接关闭处理函数
func (c *client) OnDisconnect(handler network.DisconnectHandler) {
	c.disconnectHandler = handler
}

// OnHeartbeat 监听心跳
// @param handler network.HeartbeatHandler 心跳处理函数
func (c *client) OnHeartbeat(handler network.HeartbeatHandler) {
	c.heartbeatHandler = handler
}

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 消息接收处理函数
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}
