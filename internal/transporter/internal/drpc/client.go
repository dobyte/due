package drpc

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"golang.org/x/sync/errgroup"
)

type Client struct {
	opts  *ClientOptions
	addr  *net.TCPAddr
	pool  sync.Pool
	idx   atomic.Uint64
	conns []*ClientConn
}

func NewClient(opts *ClientOptions) (*Client, error) {
	addr, err := net.ResolveTCPAddr("tcp", opts.Addr)
	if err != nil {
		return nil, err
	}

	c := &Client{}
	c.addr = addr
	c.opts = opts
	c.pool = sync.Pool{New: func() any { return &message{call: make(chan buffer.Buffer, 1)} }}
	c.conns = make([]*ClientConn, 0, c.opts.ConnNum)

	return c, nil
}

// Establish 新建连接
func (c *Client) Establish() error {
	num := c.opts.ConnNum

	for {
		conns, err := c.doEstablish(num)
		if err != nil {
			log.Warnf("doEstablish failed: %v", err)
			continue
		}

		c.conns = append(c.conns, conns...)

		num -= len(conns)

		if num <= 0 {
			break
		}
	}

	return nil
}

// 新建连接
func (c *Client) doEstablish(num int) ([]*ClientConn, error) {
	var (
		mu    sync.Mutex
		eg, _ = errgroup.WithContext(context.Background())
		conns = make([]*ClientConn, 0, num)
	)

	for range num {
		eg.Go(func() error {
			conn := newClientConn(c)

			if err := conn.dial(); err != nil {
				return err
			}

			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil && len(conns) == 0 {
		return nil, err
	}

	return conns, nil
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
	msg.state.Store(statePending)

	if err := conn.send(msg); err != nil {
		c.release(msg, true)
		return nil, err
	}

	if c.opts.CallTimeout > 0 {
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
	} else {
		select {
		case <-ctx.Done():
			conn.delete(msg)
			return nil, ctx.Err()
		case res, ok := <-msg.call:
			if !ok {
				return nil, errors.ErrConnectionHanged
			}

			return res, nil
		}
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
func (c *Client) load(idx ...int64) *ClientConn {
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

	if len(isNeedClose) > 0 && isNeedClose[0] {
		// 排空 channel 以便复用（超时/取消场景下读 goroutine 可能已写入）
		select {
		case buf := <-msg.call:
			buf.Release()
		default:
		}
	}

	c.pool.Put(msg)
}
