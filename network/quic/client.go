package quic

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/quic-go/quic-go"
)

type client struct {
	opts              *clientOptions            // 配置
	id                atomic.Int64              // 连接ID
	tlsConfig         *tls.Config               // TLS配置
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
	taskPool          sync.Pool                 // 任务对象池
}

var _ network.Client = &client{}

func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &client{}
	c.opts = o
	c.taskPool = sync.Pool{New: func() any { return &task{} }}

	return c
}

// Dial 拨号连接
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var address string
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	if c.tlsConfig == nil {
		c.tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"due-quic"},
		}
		if c.opts.serverName != "" {
			c.tlsConfig.ServerName = c.opts.serverName
		}
	}

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.opts.dialTimeout)
	defer cancel()

	qc, err := quic.DialAddr(ctx, udpAddr.String(), c.tlsConfig, &quic.Config{
		MaxIdleTimeout:       c.opts.heartbeatInterval * 3,
		KeepAlivePeriod:      c.opts.heartbeatInterval / 2,
		HandshakeIdleTimeout: c.opts.dialTimeout,
		EnableDatagrams:      false,
		Allow0RTT:            false,
	})
	if err != nil {
		return nil, err
	}

	stream, err := qc.OpenStreamSync(context.Background())
	if err != nil {
		_ = qc.CloseWithError(0, "open stream failed")
		return nil, err
	}

	return newClientConn(c.id.Add(1), qc, stream, c), nil
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

// 分配任务对象
func (c *client) allocateTask(typ int8, msg ...[]byte) *task {
	t := c.taskPool.Get().(*task)
	t.typ = typ
	if len(msg) > 0 {
		t.msg = msg[0]
	}

	return t
}

// 回收任务到对象池
func (c *client) recycleTask(t *task) {
	t.msg = nil
	c.taskPool.Put(t)
}
