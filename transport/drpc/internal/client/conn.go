package client

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"net"
	"sync"
)

var reader = packet.NewReader()

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

	go c.write()

	go c.read()

	return c
}

// 发送请求
func (c *Conn) send(ctx context.Context, seq uint64, buf *packet.Buffer) *Call {
	call := &Call{data: make(chan []byte)}

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

			c.pending[ch.seq] = ch.call

			_, err := conn.Write(ch.buf.Bytes())
			ch.buf.Recycle()

			if err != nil {
				continue
			}
		}
	}
}

func (c *Conn) read() {
	conn := c.conn

	for {
		isHeartbeat, _, seq, data, err := reader.ReadMessage(conn)
		if err != nil {
			// TODO：处理错误
			return
		}

		fmt.Println(seq)

		if isHeartbeat {
			continue
		}

		call, ok := c.pending[seq]
		if !ok {
			continue
		}

		delete(c.pending, seq)

		call.data <- data
	}
}
