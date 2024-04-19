package client

import (
	"context"
	"errors"
	endpoints "github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"net"
	"sync/atomic"
)

type Client struct {
	chWrite chan chWrite
	conns   []*Conn
	stream  *Conn
	index   int64
}

func NewClient(ep *endpoints.Endpoint) *Client {
	c := &Client{}
	c.chWrite = make(chan chWrite, 4096)
	c.conns = make([]*Conn, 0, 5)

	conn, err := net.Dial("tcp", ep.Address())
	if err != nil {
		panic(err.Error())
	}

	c.stream = newConn(conn)

	for i := 0; i < 500; i++ {
		conn, err = net.Dial("tcp", ep.Address())
		if err != nil {
			i--
			continue
		}

		c.conns = append(c.conns, newConn(conn))
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

// Push 推送消息
func (c *Client) Push(ctx context.Context, seq uint64, buf *packet.Buffer, data []byte) ([]byte, error) {
	//c.stream.send(ctx, seq, buf, data)

	index := atomic.AddInt64(&c.index, 1) % int64(len(c.conns))

	c.conns[index].send(ctx, seq, buf, data)

	return nil, nil

	//call := c.stream.send(ctx, seq, buf, data)
	//
	//select {
	//case <-ctx.Done():
	//	return nil, errors.New("timeout")
	//case res := <-call.Done():
	//	return res, nil
	//}
}
