package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/internal/client"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"sync/atomic"
)

type Client struct {
	seq uint64
	cli *client.Client
}

func NewClient(cli *client.Client) *Client {
	return &Client{
		cli: cli,
	}
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, event cluster.Event, cid, uid int64) (bool, error) {
	seq := atomic.AddUint64(&c.seq, 1)

	buf := protocol.EncodeTriggerReq(seq, event, cid, uid)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return false, err
	}

	code, err := protocol.DecodeTriggerRes(res)
	if err != nil {
		return false, err
	}

	return code == codes.NotFoundSession, nil
}

// AsyncTrigger 异步触发事件
func (c *Client) AsyncTrigger(ctx context.Context, event cluster.Event, cid, uid int64) error {
	return c.cli.Send(ctx, protocol.EncodeTriggerReq(0, event, cid, uid))
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, cid, uid int64, message []byte) (bool, error) {
	seq := atomic.AddUint64(&c.seq, 1)

	buf := protocol.EncodeDeliverReq(seq, cid, uid, message)

	res, err := c.cli.Call(ctx, seq, buf, cid)
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
	return c.cli.Send(ctx, protocol.EncodeDeliverReq(0, cid, uid, message), cid)
}
