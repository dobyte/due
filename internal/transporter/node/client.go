package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/internal/client"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
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
func (c *Client) Trigger(ctx context.Context, event cluster.Event, cid, uid int64) error {
	return c.cli.Send(ctx, protocol.EncodeTriggerReq(0, event, cid, uid))
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, cid, uid int64, message []byte) error {
	return c.cli.Send(ctx, protocol.EncodeDeliverReq(0, cid, uid, message), cid)
}
