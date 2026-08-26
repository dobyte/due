package kcp

import (
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/xtaci/kcp-go/v5"
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
	c.taskPool = sync.Pool{New: func() any { return &task{} }}

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

	conn, err := kcp.DialWithOptions(address, nil, 10, 3)
	if err != nil {
		return nil, err
	}

	return newClientConn(c.id.Add(1), conn, c), nil
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

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 接收消息hook函数
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}

// allocateTask 分配任务对象
// 从对象池中取出任务并填充类型与消息内容
// @param typ int8 任务类型
// @param msg ...[]byte 待发送的消息字节，可选
// @return @1 *task 分配到的任务对象
func (c *client) allocateTask(typ int8, msg ...[]byte) *task {
	t := c.taskPool.Get().(*task)
	t.typ = typ
	if len(msg) > 0 {
		t.msg = msg[0]
	}

	return t
}

// recycleTask 回收任务到对象池
// 清空消息内容后将任务归还对象池，以便复用
// @param t *task 待回收的任务对象
func (c *client) recycleTask(t *task) {
	t.msg = nil
	c.taskPool.Put(t)
}
