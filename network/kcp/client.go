package kcp

import (
	"github.com/dobyte/due/v2/network"
	"github.com/xtaci/kcp-go/v5"
	"sync/atomic"
)

type client struct {
	opts              *clientOptions            // 配置
	id                int64                     // 连接ID
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Client = &client{}

func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &client{opts: o}
}

// Dial 拨号连接
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var address string
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	//key := pbkdf2.Key([]byte("demo pass"), []byte("demo salt"), 1024, 32, sha1.New)
	//block, _ := kcp.NewAESBlockCrypt(key)

	conn, err := kcp.DialWithOptions(address, nil, 10, 3)
	if err != nil {
		return nil, err
	}

	return newClientConn(c, atomic.AddInt64(&c.id, 1), conn), nil
}

// Protocol 协议
func (c *client) Protocol() string {
	return protocol
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
