package node

import (
	"context"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/internal/stream/internal/client"
	"github.com/dobyte/due/v2/internal/stream/internal/codes"
	"github.com/dobyte/due/v2/internal/stream/internal/protocol"
	"sync/atomic"
)

type Client struct {
	seq    uint64
	client *client.Client
}

func NewClient(ep *endpoint.Endpoint) *Client {
	c := &Client{}

	return c
}

func (c *Client) doGenSeq() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, cid, uid int64, message []byte) (bool, error) {
	seq := c.doGenSeq()

	buf := protocol.EncodeDeliverReq(seq, cid, uid, message)

	res, err := c.client.Call(ctx, seq, buf)
	if err != nil {
		return false, err
	}

	code, err := protocol.DecodeDeliverRes(res)
	if err != nil {
		return false, err
	}

	return code == codes.NotFoundSession, nil
}

// AsyncDeliver 异步投递消息
func (c *Client) AsyncDeliver(ctx context.Context, cid, uid int64, message []byte) error {
	buf := protocol.EncodeDeliverReq(0, cid, uid, message)

	return c.client.Send(ctx, buf)
}
