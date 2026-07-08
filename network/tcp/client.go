package tcp

import (
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"

	ctls "github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/network"
)

type client struct {
	opts              *clientOptions            // 配置
	id                atomic.Int64              // 连接ID
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
	var (
		conn    net.Conn
		address string
	)

	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	if c.opts.caFile != "" {
		config, err := ctls.MakeTCPClientTLSConfig(c.opts.caFile, c.opts.serverName)
		if err != nil {
			return nil, err
		}

		dialer := &net.Dialer{Timeout: c.opts.dialTimeout}

		if conn, err = tls.DialWithDialer(dialer, tcpAddr.Network(), tcpAddr.String(), config); err != nil {
			return nil, err
		}
	} else {
		if conn, err = net.DialTimeout(tcpAddr.Network(), tcpAddr.String(), c.opts.dialTimeout); err != nil {
			return nil, err
		}
	}

	conn.(*net.TCPConn).SetNoDelay(true)

	return newClientConn(c.id.Add(1), conn, c), nil
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
