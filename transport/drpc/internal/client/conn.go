package client

import (
	"context"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"net"
	"sync"
)

var reader = packet.NewReader()

type Conn struct {
	conn    net.Conn
	chWrite chan chWrite
	//rw      sync.RWMutex
	//pending map[uint64]*Call
	pending sync.Map
}

func newConn(conn net.Conn, ch ...chan chWrite) *Conn {
	c := &Conn{}
	c.conn = conn
	//c.pending = make(map[uint64]*Call)

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
func (c *Conn) send(ctx context.Context, seq uint64, buf *packet.Buffer, data []byte) *Call {
	call := &Call{data: make(chan []byte)}

	c.chWrite <- chWrite{
		ctx:  ctx,
		seq:  seq,
		buf:  buf,
		call: call,
		data: data,
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

			//c.pending.Store(ch.seq, ch.call)

			_, err := conn.Write(ch.buf.Bytes())
			ch.buf.Recycle()

			if err != nil {
				continue
			}

			if len(ch.data) > 0 {
				if _, err = conn.Write(ch.data); err != nil {
					continue
				}
			}
		}
	}
}

func (c *Conn) read() {
	conn := c.conn

	for {
		isHeartbeat, _, _, _, err := reader.ReadMessage(conn)
		//isHeartbeat, _, seq, data, err := reader.ReadMessage(conn)
		if err != nil {
			// TODO：处理错误
			return
		}

		if isHeartbeat {
			continue
		}

		//v, ok := c.pending.Load(seq)
		//if !ok {
		//	continue
		//}
		//
		//c.pending.Delete(seq)
		//
		//v.(*Call).data <- data
	}
}
