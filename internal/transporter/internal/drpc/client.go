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
	epoch atomic.Uint64
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

// Establish establishes the configured number of connections.
//
// It tries repeatedly until the requested number of connections is reached. Failures are retried
// with exponential backoff to avoid busy waiting, and the wait can be canceled through the
// optional context.
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

// doEstablish establishes up to num connections concurrently.
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
				conn.destroy()
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

// Close closes every connection of the client and wakes up all waiters.
//
// The connection slice no longer changes after Establish completes, so it is kept intact here and
// only each connection is marked as closed. This keeps concurrent Call and Send safe while they
// load a connection, and a closed connection rejects new sends through its closed flag.
func (c *Client) Close() error {
	for _, conn := range c.conns {
		if conn != nil {
			conn.closed.Store(true)
			conn.destroy()
		}
	}
	return nil
}

// Call sends a request and waits for its response.
func (c *Client) Call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer, idx ...int64) (buffer.Buffer, error) {
	if conn, err := c.doLoadConn(ctx, idx...); err != nil {
		buf.Release()
		return nil, err
	} else {
		return conn.call(ctx, seq, buf)
	}
}

// Push sends a message without waiting for a response.
func (c *Client) Push(ctx context.Context, buf *buffer.NocopyBuffer, idx ...int64) error {
	if conn, err := c.doLoadConn(ctx, idx...); err != nil {
		buf.Release()
		return err
	} else {
		return conn.push(buf)
	}
}

// doLoadConn returns a connection, selected by idx or in round-robin order.
func (c *Client) doLoadConn(ctx context.Context, idx ...int64) (*ClientConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if n := len(c.conns); n > 0 {
		if len(idx) > 0 && idx[0] >= 0 {
			return c.conns[idx[0]%int64(n)], nil
		} else {
			return c.conns[(c.idx.Add(1)-1)%uint64(n)], nil
		}
	}

	return nil, errors.ErrClientClosed
}

// doGenEpoch generates a connection epoch, skipping the value 0.
func (c *Client) doGenEpoch() uint64 {
	if epoch := c.epoch.Add(1); epoch == 0 {
		return c.epoch.Add(1)
	} else {
		return epoch
	}
}
