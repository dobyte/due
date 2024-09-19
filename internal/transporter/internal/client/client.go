package client

import (
	"context"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ordered   = 1 // 有序连接数
	unordered = 1 // 无序连接数
)

const (
	defaultTimeout = 3 * time.Second // 调用超时时间
)

type chWrite struct {
	ctx  context.Context // 上下文
	seq  uint64          // 序列号
	buf  buffer.Buffer   // 数据Buffer
	call chan []byte     // 回调数据
}

type Client struct {
	opts        *Options       // 配置
	chWrite     chan *chWrite  // 写入队列
	connections []*Conn        // 连接
	wg          sync.WaitGroup // 等待组
	closed      atomic.Bool    // 已关闭
}

func NewClient(opts *Options) *Client {
	c := &Client{}
	c.opts = opts
	c.chWrite = make(chan *chWrite, 10240)
	c.connections = make([]*Conn, 0, ordered+unordered)
	c.init()

	return c
}

// Call 调用
func (c *Client) Call(ctx context.Context, seq uint64, buf buffer.Buffer, idx ...int64) ([]byte, error) {
	if c.closed.Load() {
		return nil, errors.ErrClientClosed
	}

	call := make(chan []byte)

	conn := c.load(idx...)

	if err := conn.send(&chWrite{
		ctx:  ctx,
		seq:  seq,
		buf:  buf,
		call: call,
	}); err != nil {
		return nil, err
	}

	ctx1, cancel1 := context.WithTimeout(ctx, defaultTimeout)
	defer cancel1()

	select {
	case <-ctx.Done():
		conn.cancel(seq)
		return nil, ctx.Err()
	case <-ctx1.Done():
		conn.cancel(seq)
		return nil, ctx1.Err()
	case data := <-call:
		return data, nil
	}
}

// Send 发送
func (c *Client) Send(ctx context.Context, buf buffer.Buffer, idx ...int64) error {
	if c.closed.Load() {
		return errors.ErrClientClosed
	}

	conn := c.load(idx...)

	return conn.send(&chWrite{
		ctx: ctx,
		buf: buf,
	})
}

// 获取连接
func (c *Client) load(idx ...int64) *Conn {
	if len(idx) > 0 {
		return c.connections[idx[0]%ordered]
	} else {
		return c.connections[ordered]
	}
}

// 新建连接
func (c *Client) init() {
	c.wg.Add(ordered + unordered)

	go c.wait()

	for i := 0; i < ordered; i++ {
		c.connections = append(c.connections, newConn(c))
	}

	for i := 0; i < unordered; i++ {
		c.connections = append(c.connections, newConn(c, c.chWrite))
	}
}

// 连接断开
func (c *Client) done() {
	c.wg.Done()
}

// 等待客户端连接全部关闭
func (c *Client) wait() {
	c.wg.Wait()
	c.closed.Store(true)
	c.connections = nil
	close(c.chWrite)

	if c.opts.CloseHandler != nil {
		c.opts.CloseHandler()
	}
}
