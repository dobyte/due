package drpc

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"golang.org/x/sync/errgroup"
)

type Client struct {
	id    atomic.Uint64
	opts  *ClientOptions
	addr  *net.TCPAddr
	idx   atomic.Uint64
	conns []*ClientConn
}

func NewClient(addr string, opts *ClientOptions) (*Client, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{}
	c.addr = tcpAddr
	c.opts = opts
	c.conns = make([]*ClientConn, 0, c.opts.ConnNum)

	return c, nil
}

// Establish 新建连接
// 循环尝试建立指定数量的连接；存在失败时采用指数退避重试，避免忙等；支持通过 ctx 取消等待
func (c *Client) Establish(ctx ...context.Context) error {
	ct := context.Background()
	if len(ctx) > 0 && ctx[0] != nil {
		ct = ctx[0]
	}

	var (
		num   = c.opts.ConnNum
		delay time.Duration
	)

	for num > 0 {
		if err := ct.Err(); err != nil {
			return err
		}

		conns, err := c.doEstablish(num)

		if len(conns) > 0 {
			delay = 0
			c.conns = append(c.conns, conns...)
			num -= len(conns)
		}

		if num <= 0 {
			break
		}

		if err != nil {
			log.Warnf("doEstablish failed: %v", err)
		}

		if delay == 0 {
			delay = 5 * time.Millisecond
		} else {
			delay *= 2
		}
		if delay > time.Second {
			delay = time.Second
		}

		select {
		case <-ct.Done():
			return ct.Err()
		case <-time.After(delay):
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

	if err := eg.Wait(); err != nil {
		return conns, err
	}

	return conns, nil
}

// Close 关闭客户端所有连接并唤醒全部等待者
// 连接切片在 Establish 完成后不再变更，此处保留切片内容（仅逐连接置关闭标记），
// 使并发的 Call/Send 经由 load 取连接时始终安全；已关闭连接会通过 closed 标记拒绝新的发送
func (c *Client) Close() error {
	for _, conn := range c.conns {
		if conn != nil {
			conn.closed.Store(true)
			conn.destroy()
		}
	}
	return nil
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

	return conn.call(ctx, seq, buf)
}

// Push 发送消息
func (c *Client) Push(ctx context.Context, buf *buffer.NocopyBuffer, idx ...int64) error {
	if err := ctx.Err(); err != nil {
		buf.Release()
		return err
	}

	conn := c.load(idx...)

	if conn == nil {
		buf.Release()
		return errors.ErrClientClosed
	}

	return conn.push(buf)
}

// 获取连接
func (c *Client) load(idx ...int64) *ClientConn {
	if n := len(c.conns); n > 0 {
		if len(idx) > 0 && idx[0] >= 0 {
			return c.conns[idx[0]%int64(n)]
		} else {
			return c.conns[(c.idx.Add(1)-1)%uint64(n)]
		}
	}

	return nil
}
