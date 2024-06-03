package client

import (
	"context"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xrand"
)

const (
	ordered   = 10 // 有序连接数
	unordered = 3  // 无序连接数
)

type chWrite struct {
	ctx  context.Context // 上下文
	seq  uint64          // 序列号
	buf  buffer.Buffer   // 数据Buffer
	call chan []byte     // 回调数据
}

type Client struct {
	opts    *Options      // 配置
	conns   []*Conn       // 连接
	chWrite chan *chWrite // 写入队列
}

func NewClient(opts *Options) (*Client, error) {
	c := &Client{}
	c.opts = opts
	c.conns = make([]*Conn, 0, ordered+unordered)
	c.chWrite = make(chan *chWrite, 4096)

	for i := 0; i < ordered; i++ {
		c.conns = append(c.conns, NewConn(c))
	}

	for i := 0; i < unordered; i++ {
		c.conns = append(c.conns, NewConn(c, c.chWrite))
	}

	return c, nil
}

// Call 调用
func (c *Client) Call(ctx context.Context, seq uint64, buf buffer.Buffer, idx ...int64) ([]byte, error) {
	call := make(chan []byte)

	conn := c.conn(idx...)

	conn.send(&chWrite{
		ctx:  ctx,
		seq:  seq,
		buf:  buf,
		call: call,
	})

	select {
	case <-ctx.Done():
		return nil, errors.New("timeout")
	case data := <-call:
		return data, nil
	}
}

// Send 发送
func (c *Client) Send(ctx context.Context, buf buffer.Buffer, idx ...int64) error {
	conn := c.conn(idx...)

	conn.send(&chWrite{
		ctx: ctx,
		buf: buf,
	})

	return nil
}

// 获取连接
func (c *Client) conn(idx ...int64) *Conn {
	if len(idx) > 0 {
		return c.conns[idx[0]%ordered]
	} else {
		return c.conns[xrand.Int(ordered, unordered+ordered-1)]
	}
}
