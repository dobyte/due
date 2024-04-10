package node

import (
	"context"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/dobyte/due/transport/kitex/v2/internal/protocol/message"
	"github.com/dobyte/due/transport/kitex/v2/internal/protocol/node"
	inner "github.com/dobyte/due/transport/kitex/v2/internal/protocol/node/node"
	"github.com/dobyte/due/v2/transport"
	"time"
)

type Client struct {
	cli inner.Client
}

func NewClient(addr string) (*Client, error) {
	cli, err := inner.NewClient("Node", client.WithHostPorts(addr), client.WithLongConnection(connpool.IdleConfig{
		MaxIdlePerAddress: 10,
		MaxIdleGlobal:     100,
		MaxIdleTimeout:    time.Minute,
		MinIdlePerAddress: 2,
	}))
	if err != nil {
		return nil, err
	}

	return &Client{cli: cli}, nil
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	_, err = c.cli.Trigger(ctx, &node.TriggerRequest{
		Event: int8(args.Event),
		GID:   args.GID,
		CID:   args.CID,
		UID:   args.UID,
	})
	return
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
	_, err = c.cli.Deliver(ctx, &node.DeliverRequest{
		GID: args.GID,
		NID: args.NID,
		CID: args.CID,
		UID: args.UID,
		Message: &message.Message{
			Seq:    args.Message.Seq,
			Route:  args.Message.Route,
			Buffer: args.Message.Buffer,
		},
	})
	return
}
