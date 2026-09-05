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
	if err := ctx.Err(); err != nil {
		buf.Release()
		return nil, err
	}

	conn := c.load(idx...)

	if conn == nil {
		buf.Release()
		return nil, errors.ErrClientClosed
	}

	call := make(chan buffer.Buffer, 1)
	conn.pending.store(seq, call)

	if err := conn.send(buf); err != nil {
		buf.Release()
		conn.pending.delete(seq)
		return nil, err
	}

	if c.opts.CallTimeout > 0 {
		tctx, tcancel := context.WithTimeout(ctx, c.opts.CallTimeout)
		defer tcancel()

		select {
		case <-ctx.Done():
			conn.pending.delete(seq)
			return nil, ctx.Err()
		case <-tctx.Done():
			conn.pending.delete(seq)
			return nil, tctx.Err()
		case res, ok := <-call:
			if !ok {
				return nil, errors.ErrConnectionHanged
			}

			return res, nil
		}
	} else {
		select {
		case <-ctx.Done():
			conn.pending.delete(seq)
			return nil, ctx.Err()
		case res, ok := <-call:
			if !ok {
				return nil, errors.ErrConnectionHanged
			}

			return res, nil
		}
	}
}

// Send 发送
func (c *Client) Send(ctx context.Context, buf *buffer.NocopyBuffer, idx ...int64) error {
	err := c.send(ctx, buf, idx...)

	if err != nil {
		buf.Release()
	}

	return err
}

// 发送
func (c *Client) send(ctx context.Context, buf *buffer.NocopyBuffer, idx ...int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if conn := c.load(idx...); conn == nil {
		return errors.ErrClientClosed
	} else {
		return conn.send(buf)
	}
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
