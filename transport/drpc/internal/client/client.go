package client

import (
	"context"
	"errors"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"net"
)

type Client struct {
	chWrite chan chWrite
	conns   map[net.Conn]*Conn
}

func NewClient() *Client {
	c := &Client{}
	c.chWrite = make(chan chWrite, 4096)
	c.conns = make(map[net.Conn]*Conn)

	for i := 0; i < 5; i++ {
		conn, err := net.Dial("tcp", "127.0.0.1:3553")
		if err != nil {
			i--
			continue
		}

		c.conns[conn] = newConn(conn, c.chWrite)
	}

	return c
}

func (c *Client) Call(ctx context.Context, seq uint64, buf *packet.Buffer) ([]byte, error) {
	call := &Call{data: make(chan []byte)}

	c.chWrite <- chWrite{
		ctx:  ctx,
		seq:  seq,
		buf:  buf,
		call: call,
	}

	select {
	case <-ctx.Done():
		return nil, errors.New("timeout")
	case data := <-call.Done():
		return data, nil
	}
}
