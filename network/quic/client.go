package quic

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/quic-go/quic-go"
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

// NewClient 创建一个QUIC客户端
// 基于 quic-go 实现，内部维护连接ID自增与任务对象池，TLS配置通过配置项或环境配置提供
// @param opts ...ClientOption 客户端配置项，可缺省，缺省时使用默认配置
// @return @1 network.Client 客户端实例
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
// 建立QUIC连接并打开一条双向流，随后创建客户端连接对象（连接Open钩子会在此时触发成功/失败）
// @param addr ...string 目标地址，格式如 host:ip；可缺省，缺省时使用配置中的连接地址
// @return @1 network.Conn 客户端连接对象
// @return @2 error 连接失败时返回的错误
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var address string
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.opts.dialTimeout)
	defer cancel()

	qc, err := quic.DialAddr(ctx, udpAddr.String(), c.opts.tlsConfig, &quic.Config{
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
// @return @1 string QUIC协议标识
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

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 接收消息hook函数
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}

// allocateTask 分配任务对象
// 从任务对象池中获取并复用任务对象，避免频繁分配
// @param typ int8 任务类型
// @param msg ...[]byte 待发送的消息字节，可缺省
// @return @1 *task 任务对象
func (c *client) allocateTask(typ int8, msg ...[]byte) *task {
	t := c.taskPool.Get().(*task)
	t.typ = typ
	if len(msg) > 0 {
		t.msg = msg[0]
	}

	return t
}

// recycleTask 回收任务到对象池
// 清理任务数据后将对象归还池中以供复用
// @param t *task 待回收的任务对象
func (c *client) recycleTask(t *task) {
	t.msg = nil
	c.taskPool.Put(t)
}
