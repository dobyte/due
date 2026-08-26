package ws

import (
	"sync"
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
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
	taskPool          sync.Pool                 // 任务对象池
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
	c.taskPool = sync.Pool{New: func() any { return &task{} }}

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

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 消息接收处理函数
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
