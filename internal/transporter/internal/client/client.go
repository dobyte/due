package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"golang.org/x/sync/errgroup"
)

const (
	defaultTimeout = 3 * time.Second // 调用超时时间
	defaultConnNum = 20              // 默认连接数
)

type chWrite struct {
	seq  uint64        // 序列号
	buf  buffer.Buffer // 数据buffer
	call chan []byte   // 回调数据
}

type Client struct {
	opts            *Options       // 配置
	connections     []*Conn        // 连接
	disorderlyQueue chan *chWrite  // 无序队列
	wg              sync.WaitGroup // 等待组
	closed          atomic.Bool    // 已关闭
	pool            sync.Pool      // 连接池
}

func NewClient(opts *Options) *Client {
	c := &Client{}
	c.opts = opts
	c.connections = make([]*Conn, 0, defaultConnNum)
	c.disorderlyQueue = make(chan *chWrite, 10240)
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
			conn := newConn(c, c.disorderlyQueue)

			if err := conn.dial(); err != nil {
				return err
			}

			mu.Lock()
			c.connections = append(c.connections, conn)
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil && len(c.connections) == 0 {
		return err
	}

	return nil
}

// Call 调用
func (c *Client) Call(ctx context.Context, seq uint64, buf buffer.Buffer, idx ...int64) ([]byte, error) {
	if c.closed.Load() {
		return nil, errors.ErrClientClosed
	}

	ch := c.pool.Get().(*chWrite)
	ch.seq = seq
	ch.buf = buf
	ch.call = make(chan []byte)

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
	case data, ok := <-ch.call:
		if !ok {
			return nil, errors.ErrConnectionHanged
		}

		return data, nil
	}
}

// Send 发送
func (c *Client) Send(ctx context.Context, buf buffer.Buffer, idx ...int64) error {
	if c.closed.Load() {
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
		return c.connections[idx[0]%int64(len(c.connections))]
	} else {
		return c.connections[0]
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

	if ch.call != nil {
		close(ch.call)
		ch.call = nil
	}

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
		close(c.disorderlyQueue)
	})

	if c.opts.CloseHandler != nil {
		c.opts.CloseHandler()
	}
}
