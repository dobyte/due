package client

import (
	"fmt"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxRetryTimes     = 5                      // 最大重试次数
	dialTimeout       = 500 * time.Millisecond // 拨号超时时间
	heartbeatInterval = 10 * time.Second       // 心跳间隔时间
)

const (
	connClosed int32 = 0 // 连接关闭
	connOpened int32 = 1 // 连接打开
)

type Conn struct {
	cli               *Client       // 客户端
	state             int32         // 连接状态
	chWrite           chan *chWrite // 写入队列
	pending           sync.Map      // 等待队列
	done              chan struct{} // 关闭请求
	builtin           bool          // 是否内建
	conn              atomic.Value  // 源连接
	lastHeartbeatTime int64         // 上次心跳时间
}

func newConn(cli *Client, ch ...chan *chWrite) *Conn {
	c := &Conn{}
	c.cli = cli
	c.done = make(chan struct{})

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
func (c *Conn) send(ch *chWrite) bool {
	if atomic.LoadInt32(&c.state) == connClosed {
		return false
	}

	c.chWrite <- ch

	return true
}

// 拨号
func (c *Conn) dial() {
	var (
		delay time.Duration
		retry int
	)

	atomic.StoreInt32(&c.state, connClosed)

	fmt.Println("dial")

	for {
		conn, err := net.DialTimeout("tcp", c.cli.opts.Addr, dialTimeout)
		if err != nil {
			retry++

			if retry >= maxRetryTimes {
				fmt.Println("destroy")
				c.destroy()
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
	fmt.Println("process")
	atomic.StoreInt32(&c.state, connOpened)

	c.done = make(chan struct{})

	c.conn.Store(conn)

	go c.read(conn)

	seq := uint64(1)

	call := make(chan []byte)

	c.pending.Store(seq, call)

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
				c.retry()
				return
			}

			if isHeartbeat {
				continue
			}

			v, ok := c.pending.Load(seq)
			if !ok {
				continue
			}

			c.pending.Delete(seq)

			v.(chan []byte) <- data
		}
	}
}

// 写入数据
func (c *Conn) write(conn net.Conn) {
	fmt.Println("write")
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			fmt.Println("write done")
			//if err := conn.Close(); err != nil {
			//	log.Warnf("close conn error: %v", err)
			//}
			return
		case <-ticker.C:
			fmt.Println(22222)
			deadline := xtime.Now().Add(-2 * heartbeatInterval).UnixNano()

			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				c.retry()
				return
			} else {
				if _, err := conn.Write(protocol.Heartbeat()); err != nil {
					log.Warnf("write heartbeat message error: %v", err)
					c.retry()
					return
				}
			}
		case ch, ok := <-c.chWrite:
			if !ok {
				return
			}

			if ch.seq != 0 {
				c.pending.Store(ch.seq, ch.call)
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
func (c *Conn) retry() {
	fmt.Println("retry")
	close(c.done)

	c.dial()
}

// 销毁连接
func (c *Conn) destroy() {
	c.cli.done()

	close(c.done)

	if c.builtin {
		close(c.chWrite)
	}
}

// 关闭连接
func (c *Conn) close() {
	fmt.Println("closed1")
	close(c.done)
	fmt.Println("closed2")

	if conn := c.conn.Load(); conn != nil {
		if err := conn.(net.Conn).Close(); err != nil {
			log.Warnf("close conn error: %v", err)
		}
	}
}
