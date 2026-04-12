package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	ID                string        // 实例ID
	Kind              cluster.Kind  // 实例类型
	ConnNum           int           // 连接数
	CallTimeout       time.Duration // 调用超时时间
	DialTimeout       time.Duration // 拨号超时时间
	DialRetryTimes    int           // 拨号重试次数
	WriteTimeout      time.Duration // 写超时时间
	WriteQueueSize    int32         // 写队列大小
	FaultRecoveryTime time.Duration // 故障恢复时间
}

type Client struct {
	addr  string        // 连接地址
	opts  *Options      // 配置
	pool  sync.Pool     // 对象池
	conns []*conn       // 连接
	idx   atomic.Uint64 // 分配连接索引
}

func NewClient(addr string, opts *Options) *Client {
	c := &Client{}
	c.addr = addr
	c.opts = opts
	c.pool = sync.Pool{New: func() any { return &message{} }}
	return c
}

// Establish 新建连接
func (c *Client) Establish() error {
	var (
		mu    sync.Mutex
		eg, _ = errgroup.WithContext(context.Background())
		conns = make([]*conn, 0, c.opts.ConnNum)
	)

	for range c.opts.ConnNum {
		eg.Go(func() error {
			conn := newConn(c)

			if err := conn.dial(); err != nil {
				log.Warnf("conn dial failed: %v", err)
				return err
			}

			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil && len(conns) == 0 {
		return err
	}

	c.conns = conns

	return nil
}

// Call 调用
func (c *Client) Call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer, idx ...int64) (buffer.Buffer, error) {
	conn := c.load(idx...)

	if conn == nil {
		buf.Release()
		return nil, errors.ErrClientClosed
	}

	msg := c.pool.Get().(*message)
	msg.seq = seq
	msg.buf = buf
	msg.call = make(chan buffer.Buffer)
	msg.state.Store(statePending)

	if err := conn.send(msg); err != nil {
		c.release(msg, true)
		return nil, err
	}

	tctx, tcancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	defer tcancel()

	select {
	case <-ctx.Done():
		conn.delete(msg)
		return nil, ctx.Err()
	case <-tctx.Done():
		conn.delete(msg)
		return nil, tctx.Err()
	case res, ok := <-msg.call:
		if !ok {
			return nil, errors.ErrConnectionHanged
		}

		return res, nil
	}
}

// Send 发送
func (c *Client) Send(ctx context.Context, buf *buffer.NocopyBuffer, idx ...int64) error {
	conn := c.load(idx...)

	if conn == nil {
		buf.Release()
		return errors.ErrClientClosed
	}

	msg := c.pool.Get().(*message)
	msg.buf = buf

	if err := conn.send(msg); err != nil {
		c.release(msg)
		return err
	}

	return nil
}

// 获取连接
func (c *Client) load(idx ...int64) *conn {
	if n := len(c.conns); n > 0 {
		if len(idx) > 0 {
			return c.conns[idx[0]%int64(n)]
		} else {
			return c.conns[c.idx.Add(1)%uint64(n)]
		}
	}

	return nil
}

// 释放
func (c *Client) release(msg *message, isNeedClose ...bool) {
	msg.seq = 0

	if msg.buf != nil {
		msg.buf.Release()
		msg.buf = nil
	}

	if msg.call != nil && len(isNeedClose) > 0 && isNeedClose[0] {
		close(msg.call)
		msg.call = nil
	}

	c.pool.Put(msg)
}
