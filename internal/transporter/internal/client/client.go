package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"golang.org/x/sync/errgroup"
)

const (
	defaultTimeout = 5 * time.Second // 调用超时时间
)

type Options struct {
	Addr          string       // 连接地址
	InsID         string       // 实例ID
	InsKind       cluster.Kind // 实例类型
	ConnNum       int          // 连接数
	FaultInterval int64        // 故障间隔时间
}

type Client struct {
	opts  *Options  // 配置
	pool  sync.Pool // 对象池
	conns []*conn   // 连接
	idx   atomic.Uint64
}

func NewClient(opts *Options) *Client {
	c := &Client{}
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
		c.release(msg)
		return nil, err
	}

	startTime := time.Now()
	tctx, tcancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer tcancel()

	select {
	// case <-ctx.Done():
	// 	conn.delete(msg)
	// 	return nil, ctx.Err()
	case <-tctx.Done():
		fmt.Printf("call timeout, seq = %d cost = %v\n", seq, time.Since(startTime).Seconds())
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
func (c *Client) release(msg *message) {
	if msg.buf != nil {
		msg.buf.Release()
		msg.buf = nil
	}

	msg.seq = 0
	msg.call = nil

	c.pool.Put(msg)
}
