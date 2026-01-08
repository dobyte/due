package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"golang.org/x/sync/errgroup"
)

const (
	defaultTimeout = 3 * time.Second // 调用超时时间
	defaultConnNum = 10              // 默认连接数
)

type chWrite struct {
	seq  uint64               // 序列号
	buf  *buffer.NocopyBuffer // 数据buffer
	call chan buffer.Buffer   // 回调数据
}

type Client struct {
	opts   *Options       // 配置
	conns  []*Conn        // 连接
	queue  chan *chWrite  // 无序队列
	wg     sync.WaitGroup // 等待组
	closed atomic.Bool    // 已关闭
	pool   sync.Pool      // 连接池
}

func NewClient(opts *Options) *Client {
	c := &Client{}
	c.opts = opts
	c.conns = make([]*Conn, 0, defaultConnNum)
	c.queue = make(chan *chWrite, 10240)
	c.closed.Store(true)
	c.pool = sync.Pool{New: func() any { return &chWrite{} }}

	return c
}

// Establish 新建连接
func (c *Client) Establish() error {
	c.wg.Add(defaultConnNum)

	go c.wait()

	var (
		mu    sync.Mutex
		eg, _ = errgroup.WithContext(context.Background())
	)

	for range defaultConnNum {
		eg.Go(func() error {
			conn := newConn(c)

			if err := conn.dial(); err != nil {
				log.Warnf("conn dial failed: %v", err)
				return err
			}

			mu.Lock()
			c.conns = append(c.conns, conn)
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil && len(c.conns) == 0 {
		return err
	}

	c.closed.Store(false)

	return nil
}

// Call 调用
func (c *Client) Call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer, idx ...int64) (buffer.Buffer, error) {
	if c.closed.Load() {
		buf.Release()
		return nil, errors.ErrClientClosed
	}

	ch := c.pool.Get().(*chWrite)
	ch.seq = seq
	ch.buf = buf
	ch.call = make(chan buffer.Buffer)

	conn := c.load(idx...)

	if err := conn.send(ch, len(idx) > 0); err != nil {
		c.release(ch)
		return nil, err
	}

	tctx, tcancel := context.WithTimeout(ctx, defaultTimeout)
	defer tcancel()

	select {
	case <-ctx.Done():
		conn.delete(seq)
		return nil, ctx.Err()
	case <-tctx.Done():
		conn.delete(seq)
		return nil, tctx.Err()
	case res, ok := <-ch.call:
		if !ok {
			return nil, errors.ErrConnectionHanged
		}

		return res, nil
	}
}

// Send 发送
func (c *Client) Send(ctx context.Context, buf *buffer.NocopyBuffer, idx ...int64) error {
	if c.closed.Load() {
		buf.Release()
		return errors.ErrClientClosed
	}

	ch := c.pool.Get().(*chWrite)
	ch.buf = buf

	conn := c.load(idx...)

	if err := conn.send(ch, len(idx) > 0); err != nil {
		c.release(ch)
		return err
	}

	return nil
}

// 获取连接
func (c *Client) load(idx ...int64) *Conn {
	if len(idx) > 0 {
		return c.conns[idx[0]%int64(len(c.conns))]
	} else {
		return c.conns[0]
	}
}

// 释放
func (c *Client) release(ch *chWrite) {
	if ch.buf == nil {
		return
	}

	ch.buf.Release()
	ch.buf = nil
	ch.seq = 0
	ch.call = nil

	c.pool.Put(ch)
}

// 连接断开
func (c *Client) done() {
	c.wg.Done()
}

// 等待客户端连接全部关闭
func (c *Client) wait() {
	c.wg.Wait()
	c.closed.Store(true)

	time.AfterFunc(time.Second, func() {
		close(c.queue)
	})

	if c.opts.CloseHandler != nil {
		c.opts.CloseHandler()
	}
}
