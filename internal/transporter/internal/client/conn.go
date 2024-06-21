package client

import (
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync/atomic"
	"time"
)

const (
	maxRetryTimes = 5                      // 最大重试次数
	dialTimeout   = 500 * time.Millisecond // 拨号超时时间
)

type Conn struct {
	cli               *Client       // 客户端
	state             int32         // 连接状态
	chWrite           chan *chWrite // 写入队列
	pending           *pending      // 等待队列
	done              chan struct{} // 关闭请求
	builtin           bool          // 是否内建
	lastHeartbeatTime int64         // 上次心跳时间
}

func newConn(cli *Client, ch ...chan *chWrite) *Conn {
	c := &Conn{}
	c.cli = cli
	c.state = def.ConnClosed
	c.pending = newPending()

	if len(ch) > 0 {
		c.chWrite = ch[0]
	} else {
		c.chWrite = make(chan *chWrite, 10240)
		c.builtin = true
	}

	c.dial()

	return c
}

// 发送
func (c *Conn) send(ch *chWrite) error {
	if atomic.LoadInt32(&c.state) == def.ConnClosed {
		return errors.ErrConnectionClosed
	}

	c.chWrite <- ch

	return nil
}

// 拨号
func (c *Conn) dial() {
	var (
		delay time.Duration
		retry int
	)

	for {
		conn, err := net.DialTimeout("tcp", c.cli.opts.Addr, dialTimeout)
		if err != nil {
			retry++

			if retry >= maxRetryTimes {
				log.Warnf("dial failed: %v", err)
				c.close()
				break
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

		c.process(conn)

		break
	}
}

// 处理连接
func (c *Conn) process(conn net.Conn) {
	atomic.StoreInt32(&c.state, def.ConnOpened)

	c.done = make(chan struct{})

	c.lastHeartbeatTime = xtime.Now().Unix()

	go c.read(conn)

	seq := uint64(1)

	call := make(chan []byte)

	c.pending.store(seq, call)

	buf := protocol.EncodeHandshakeReq(seq, c.cli.opts.InsKind, c.cli.opts.InsID)

	defer buf.Release()

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return
	}

	<-call

	go c.write(conn)
}

// 读取数据
func (c *Conn) read(conn net.Conn) {
	for {
		select {
		case <-c.done:
			return
		default:
			isHeartbeat, _, seq, data, err := protocol.ReadMessage(conn)
			if err != nil {
				c.retry(conn)
				return
			}

			atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().Unix())

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
		case <-c.done:
			return
		case <-ticker.C:
			deadline := xtime.Now().Add(-2 * def.HeartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				c.retry(conn)
				return
			} else {
				if _, err := conn.Write(protocol.Heartbeat()); err != nil {
					log.Warnf("write heartbeat message error: %v", err)
					c.retry(conn)
					return
				}
			}
		case ch, ok := <-c.chWrite:
			if !ok {
				return
			}

			if ch.seq != 0 {
				c.pending.store(ch.seq, ch.call)
			}

			ch.buf.Range(func(node *buffer.NocopyNode) bool {
				if _, err := conn.Write(node.Bytes()); err != nil {
					return false
				} else {
					return true
				}
			})

			ch.buf.Release()
		}
	}
}

// 重试拨号
func (c *Conn) retry(conn net.Conn) {
	if !atomic.CompareAndSwapInt32(&c.state, def.ConnOpened, def.ConnRetrying) {
		return
	}

	_ = conn.Close()

	close(c.done)

	c.dial()
}

// 关闭连接
func (c *Conn) close() {
	c.cli.done()

	atomic.StoreInt32(&c.state, def.ConnClosed)

	if c.builtin {
		close(c.chWrite)
	}
}

// 取消回调
func (c *Conn) cancel(seq uint64) {
	c.pending.delete(seq)
}
