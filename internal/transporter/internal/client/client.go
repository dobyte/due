package client

import (
	"context"
	"sync"
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
	opts    *Options       // 配置
	wg      sync.WaitGroup // 等待组
	pool    sync.Pool      // 连接池
	rw      sync.RWMutex   // 读写锁
	conns   []*Conn        // 连接
	queue   chan *chWrite  // 无序队列
	closed  bool           // 已关闭
	pending *pending       // 等待队列
}

func NewClient(opts *Options) *Client {
	c := &Client{}
	c.opts = opts
	c.pool = sync.Pool{New: func() any { return &chWrite{} }}
	c.conns = make([]*Conn, 0, defaultConnNum)
	c.queue = make(chan *chWrite, 10240)
	c.closed = true
	c.pending = newPending()

	return c
}

// Establish 新建连接
func (c *Client) Establish() error {
	c.wg.Add(defaultConnNum)

	go c.wait()

	eg, _ := errgroup.WithContext(context.Background())

	for range defaultConnNum {
		eg.Go(func() error {
			conn := newConn(c)

			if err := conn.dial(); err != nil {
				log.Warnf("conn dial failed: %v", err)
				return err
			}

			c.rw.Lock()
			c.conns = append(c.conns, conn)
			c.closed = false
			c.rw.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil && len(c.conns) == 0 {
		return err
	}

	return nil
}

// Call 调用
func (c *Client) Call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer, idx ...int64) (buffer.Buffer, error) {
	ch := c.pool.Get().(*chWrite)
	ch.seq = seq
	ch.buf = buf
	ch.call = make(chan buffer.Buffer)

	pending, err := c.send(ch, idx...)
	if err != nil {
		c.release(ch)
		return nil, err
	}

	tctx, tcancel := context.WithTimeout(ctx, defaultTimeout)
	defer tcancel()

	select {
	case <-ctx.Done():
		if pending != nil {
			pending.delete(seq)
		}
		return nil, ctx.Err()
	case <-tctx.Done():
		if pending != nil {
			pending.delete(seq)
		}
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
	ch := c.pool.Get().(*chWrite)
	ch.buf = buf

	if _, err := c.send(ch, idx...); err != nil {
		c.release(ch)
		return err
	}

	return nil
}

// 发送到队列
func (c *Client) send(ch *chWrite, idx ...int64) (*pending, error) {
	if len(idx) > 0 {
		if conn, err := c.load(idx...); err != nil {
			return nil, err
		} else {
			return conn.send(ch)
		}
	} else {
		c.rw.RLock()
		if c.closed {
			c.rw.RUnlock()
			return nil, errors.ErrClientClosed
		} else {
			c.queue <- ch
			c.rw.RUnlock()
		}

		if ch.seq != 0 {
			c.pending.store(ch.seq, ch.call)

			return c.pending, nil
		} else {
			return nil, nil
		}
	}
}

// 获取连接
func (c *Client) load(idx ...int64) (*Conn, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.closed {
		return nil, errors.ErrClientClosed
	}

	if len(idx) > 0 {
		return c.conns[idx[0]%int64(len(c.conns))], nil
	} else {
		return c.conns[0], nil
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

	c.rw.Lock()
	c.closed = true
	for ch := range c.queue {
		ch.buf.Release()
	}
	close(c.queue)
	c.rw.Unlock()

	if c.opts.CloseHandler != nil {
		c.opts.CloseHandler()
	}
}
