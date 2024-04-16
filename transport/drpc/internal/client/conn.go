package client

import (
	"context"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"net"
	"sync"
)

type Conn struct {
	conn    net.Conn
	chWrite chan chWrite
	rw      sync.RWMutex
	pending map[uint64]*Call
}

func newConn(conn net.Conn, ch ...chan chWrite) *Conn {
	c := &Conn{}
	c.conn = conn
	c.pending = make(map[uint64]*Call)

	if len(ch) > 0 && ch[0] != nil {
		c.chWrite = ch[0]
	} else {
		c.chWrite = make(chan chWrite, 4096)
	}

	return c
}

// 发送请求
func (c *Conn) send(ctx context.Context, seq uint64, buf *packet.Buffer) *Call {
	call := &Call{done: make(chan struct{})}

	c.chWrite <- chWrite{
		ctx:  ctx,
		seq:  seq,
		buf:  buf,
		call: call,
	}

	return call
}

// 执行写入操作
func (c *Conn) write() {
	conn := c.conn

	for {
		select {
		case ch, ok := <-c.chWrite:
			if !ok {
				return
			}

			_, err := conn.Write(ch.buf.Bytes())
			ch.buf.Recycle()

			if err != nil {
				continue
			}

		}
	}
}
