package node

import (
	"context"
	"github.com/dobyte/due/transport/rpcx/v2/internal/code"
	"github.com/dobyte/due/transport/rpcx/v2/internal/protocol"
	"github.com/dobyte/due/v2/transport"
	cli "github.com/smallnest/rpcx/client"
)

type Client struct {
	cli *cli.OneClient
}

func NewClient(cli *cli.OneClient) *Client {
	return &Client{cli: cli}
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	req := &protocol.TriggerRequest{Event: args.Event, GID: args.GID, CID: args.CID, UID: args.UID}
	reply := &protocol.TriggerReply{}
	err = c.cli.Call(ctx, ServicePath, serviceTriggerMethod, req, reply)
	miss = reply.Code == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
	req := &protocol.DeliverRequest{GID: args.GID, NID: args.NID, CID: args.CID, UID: args.UID, Message: args.Message}
	reply := &protocol.DeliverReply{}
	err = c.cli.Call(ctx, ServicePath, serviceDeliverMethod, req, reply)
	miss = reply.Code == code.NotFoundSession

	return
}
