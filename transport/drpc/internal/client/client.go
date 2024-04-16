package client

import (
	"context"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
)

type Client struct {
	chWrite chan chWrite
}

func NewClient() *Client {
	return &Client{
		pending: make(map[uint64]*Call),
	}
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

	case data := <-call.Done():
		return data, nil
	}

}
