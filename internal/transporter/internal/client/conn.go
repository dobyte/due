package client

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
)

const (
	maxRetryTimes = 3                      // 最大重试次数
	dialTimeout   = 500 * time.Millisecond // 拨号超时时间
)

type Conn struct {
	cli               *Client            // 客户端
	state             atomic.Int32       // 连接状态
	pending           *pending           // 等待队列
	orderlyQueue      chan *chWrite      // 有序队列
	disorderlyQueue   chan *chWrite      // 无序队列
	ctx               context.Context    // 上下文
	cancel            context.CancelFunc // 取消函数
	lastHeartbeatTime atomic.Int64       // 上次心跳时间
}

func newConn(cli *Client, queue chan *chWrite) *Conn {
	c := &Conn{}
	c.cli = cli
	c.state.Store(def.ConnHanged)
	c.pending = newPending()
	c.orderlyQueue = make(chan *chWrite, 4096)
	c.disorderlyQueue = queue

	return c
}

// 拨号
func (c *Conn) dial() error {
	var (
		delay time.Duration
		retry int
	)

	for {
		conn, err := net.DialTimeout("tcp", c.cli.opts.Addr, dialTimeout)
		if err != nil {
			retry++

			if retry >= maxRetryTimes {
				c.close()
				return err
			} else {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}

				if delay > time.Second {
					delay = time.Second
				}

				time.Sleep(delay)
				continue
			}
		}

		return c.process(conn)
	}
}

// 发送
func (c *Conn) send(ch *chWrite, isOrderly ...bool) error {
	switch c.state.Load() {
	case def.ConnClosed:
		return errors.ErrConnectionClosed
	case def.ConnHanged:
		return errors.ErrConnectionHanged
	default:
		// ignore
	}

	if len(isOrderly) > 0 && isOrderly[0] {
		c.orderlyQueue <- ch
	} else {
		c.disorderlyQueue <- ch
	}

	return nil
}

// 处理连接
func (c *Conn) process(conn net.Conn) error {
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.state.Store(def.ConnOpened)
	c.lastHeartbeatTime.Store(xtime.Now().Unix())

	go c.read(conn)

	if err := c.handshake(conn); err != nil {
		c.close()

		return err
	} else {
		go c.write(conn)

		return nil
	}
}

// 握手
func (c *Conn) handshake(conn net.Conn) error {
	var (
		seq  = uint64(1)
		call = make(chan []byte)
	)

	buf := protocol.EncodeHandshakeReq(seq, c.cli.opts.InsKind, c.cli.opts.InsID)
	defer buf.Release()

	c.pending.store(seq, call)

	defer close(call)

	if _, err := conn.Write(buf.Bytes()); err != nil {
		c.pending.delete(seq)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-call:
		return nil
	}
}

// 读取数据
func (c *Conn) read(conn net.Conn) {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			isHeartbeat, _, seq, data, err := protocol.ReadMessage(conn)
			if err != nil {
				c.retry(conn)
				return
			}

			c.lastHeartbeatTime.Store(xtime.Now().Unix())

			if isHeartbeat {
				continue
			}

			call, ok := c.pending.extract(seq)
			if !ok {
				continue
			}

			call <- data
		}
	}
}

// 写入数据
func (c *Conn) write(conn net.Conn) {
	ticker := time.NewTicker(def.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case t, ok := <-ticker.C:
			if !ok {
				return
			}

			deadline := t.Add(-2 * def.HeartbeatInterval).Unix()

			if c.lastHeartbeatTime.Load() < deadline {
				log.Warn("connection heartbeat timeout")
				c.retry(conn)
				return
			} else {
				if _, err := conn.Write(protocol.Heartbeat()); err != nil {
					log.Warnf("write heartbeat message error: %v", err)
					c.retry(conn)
					return
				}
			}
		case ch, ok := <-c.orderlyQueue: // 有序队列
			if !ok {
				return
			}

			if ok = c.doWrite(conn, ch); !ok {
				return
			}
		case ch, ok := <-c.disorderlyQueue: // 无序队列
			if !ok {
				return
			}

			if ok = c.doWrite(conn, ch); !ok {
				return
			}
		}
	}
}

// 执行写入数据
func (c *Conn) doWrite(conn net.Conn, ch *chWrite) bool {
	if ch.seq != 0 {
		c.pending.store(ch.seq, ch.call)
	}

	ok := ch.buf.Visit(func(node *buffer.NocopyNode) bool {
		if _, err := conn.Write(node.Bytes()); err != nil {
			return false
		} else {
			return true
		}
	})

	c.cli.release(ch)

	if !ok {
		c.retry(conn)
	}

	return ok
}

// 重试拨号
func (c *Conn) retry(conn net.Conn) {
	if !c.state.CompareAndSwap(def.ConnOpened, def.ConnHanged) {
		return
	}

	_ = conn.Close()

	if c.cancel != nil {
		c.cancel()
	}

	if err := c.dial(); err != nil {
		log.Warnf("retry dial failed: %v", err)
	}
}

// 关闭连接
func (c *Conn) close() {
	if c.state.Swap(def.ConnClosed) == def.ConnClosed {
		return
	}

	c.cli.done()

	if c.cancel != nil {
		c.cancel()
	}

	time.AfterFunc(time.Second, func() {
		close(c.orderlyQueue)
	})
}

// 取消回调
func (c *Conn) delete(seq uint64) {
	c.pending.delete(seq)
}
