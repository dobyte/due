package node

import (
	"context"
	endpoints "github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/transport/drpc/internal/client"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"sync/atomic"
)

type Client struct {
	seq           uint64
	client        *client.Client
	deliverPacker *packet.DeliverPacker
}

func NewClient(ep *endpoints.Endpoint) *Client {
	c := &Client{}
	c.client = client.NewClient(ep)
	c.deliverPacker = packet.NewDeliverPacker()

	return c
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	return
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, args *transport.DeliverArgs) (bool, error) {
	seq := atomic.AddUint64(&c.seq, 1)

	buf, err := c.deliverPacker.PackReq(seq, args.GID, args.CID, args.UID, args.Message)
	if err != nil {
		return false, err
	}

	_, err = c.client.Push(ctx, seq, buf, args.Message.Buffer)
	if err != nil {
		return false, err
	}

	return false, nil

	//code, err := c.deliverPacker.UnpackRes(data)
	//if err != nil {
	//	return false, err
	//}
	//
	//return code == codes.NotFoundSession, nil
}
