package client

import (
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"net"
	"sync"
	"time"
)

const (
	maxRetryTimes = 5
)

type Conn struct {
	client  *Client      // 客户端
	chWrite chan chWrite // 写入队列
	pending sync.Map     // 等待队列
}

func NewConn(client *Client, ch ...chan chWrite) *Conn {
	c := &Conn{}
	c.client = client

	if len(ch) > 0 {
		c.chWrite = ch[0]
	} else {
		c.chWrite = make(chan chWrite, 4096)
	}

	return c
}

// 发送
func (c *Conn) send(ch chWrite) {
	c.chWrite <- ch
}

// 拨号
func (c *Conn) dial() {
	var (
		delay time.Duration
		retry int
	)

	for {
		conn, err := net.Dial("tcp", c.client.opts.Addr)
		if err != nil {
			retry++

			if retry >= maxRetryTimes {
				log.Warnf("client dial error: %v; more than %d retries", err, retry)
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

				log.Warnf("client dial error: %v; retrying in %v", err, delay)
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
	go c.read(conn)

	seq := uint64(1)

	cc := make(chan []byte)

	c.pending.Store(seq, cc)

	buf := protocol.EncodeHandshakeReq(seq, c.client.opts.InsKind, c.client.opts.InsID)

	defer buf.Release()

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return
	}

	<-cc

	go c.write(conn)
}

// 读取数据
func (c *Conn) read(conn net.Conn) {
	for {
		isHeartbeat, _, seq, data, err := protocol.ReadMessage(conn)
		if err != nil {
			c.dial()
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

// 写入数据
func (c *Conn) write(conn net.Conn) {
	for {
		select {
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
