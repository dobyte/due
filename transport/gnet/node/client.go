package node

import (
	"context"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/gnet/internal/pb"
	"github.com/symsimmy/due/transport/gnet/tcp"
)

type Client struct {
	client *tcp.Client
}

func NewClient(cc *tcp.Client) *Client {
	return &Client{client: cc}
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	req := &pb.TriggerRequest{
		Event: int32(args.Event),
		GID:   args.GID,
		CID:   args.CID,
		UID:   args.UID,
	}

	err = c.client.Send(transport.Trigger, req)
	miss = false
	return
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
	req := &pb.DeliverRequest{
		GID: args.GID,
		NID: args.NID,
		CID: args.CID,
		UID: args.UID,
		Message: &pb.Message{
			Seq:      args.Message.Seq,
			Route:    args.Message.Route,
			Buffer:   args.Message.Buffer,
			Compress: false,
		},
	}

	err = c.client.Send(transport.Deliver, req)
	miss = false
	return
}
