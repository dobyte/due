package kcp

import (
	"net"
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/xtaci/kcp-go/v5"
)

type client struct {
	opts              *clientOptions            // 配置
	cid               atomic.Int64              // 连接ID
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	heartbeatHandler  network.HeartbeatHandler  // 连接心跳hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Client = &client{}

// NewClient 创建KCP客户端
// 按用户传入的选项覆盖默认配置，并初始化底层客户端对象
// @param opts ...ClientOption 客户端配置选项
// @return @1 network.Client KCP客户端实例
func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &client{}
	c.opts = o

	return c
}

// Dial 拨号连接
// 未指定地址时使用客户端配置中的连接地址建立KCP会话
// @param addr ...string 目标服务器地址；缺省时使用配置项addr
// @return @1 network.Conn KCP连接
// @return @2 error 拨号失败时返回的错误
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var address string
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	udpConn, err := (&net.Dialer{Timeout: c.opts.dialTimeout}).Dial("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := kcp.NewConn(address, nil, 0, 0, udpConn.(net.PacketConn))
	if err != nil {
		_ = udpConn.Close()
		return nil, err
	}

	return newClientConn(c, conn), nil
}

// Protocol 获取协议名称
// @return @1 string 协议名称"kcp"
func (c *client) Protocol() string {
	return protocol
}

// OnConnect 监听连接打开
// @param handler network.ConnectHandler 连接打开hook函数
func (c *client) OnConnect(handler network.ConnectHandler) {
	c.connectHandler = handler
}

// OnDisconnect 监听连接关闭
// @param handler network.DisconnectHandler 连接关闭hook函数
func (c *client) OnDisconnect(handler network.DisconnectHandler) {
	c.disconnectHandler = handler
}

// OnHeartbeat 监听心跳
// @param handler network.HeartbeatHandler 心跳处理函数
func (c *client) OnHeartbeat(handler network.HeartbeatHandler) {
	c.heartbeatHandler = handler
}

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 接收消息hook函数
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}

// genConnID 生成连接ID
// @return @1 int64 连接ID
func (c *client) genConnID() int64 {
	if cid := c.cid.Add(1); cid == 0 {
		return c.cid.Add(1)
	} else {
		return cid
	}
}
