package node

import (
	"context"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/internal/code"
	"github.com/dobyte/due/transport/rpcx/internal/protocol"
	cli "github.com/smallnest/rpcx/client"
	proto "github.com/smallnest/rpcx/protocol"
	"sync"
)

var clients sync.Map

type client struct {
	client cli.XClient
}

func NewClient(ep *router.Endpoint) (*client, error) {
	cc, ok := clients.Load(ep.Address())
	if ok {
		return cc.(*client), nil
	}

	discovery, err := cli.NewPeer2PeerDiscovery("tcp@"+ep.Address(), "")
	if err != nil {
		return nil, err
	}

	option := cli.DefaultOption
	option.CompressType = proto.Gzip

	c := &client{client: cli.NewXClient(
		servicePath,
		cli.Failtry,
		cli.RandomSelect,
		discovery,
		option,
	)}
	clients.Store(ep.Address(), c)

	return c, nil
}

// Trigger 触发事件
func (c *client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	req := &protocol.TriggerRequest{Event: args.Event, GID: args.GID, UID: args.UID}
	reply := &protocol.TriggerReply{}
	err = c.client.Call(ctx, serviceTriggerMethod, req, reply)
	miss = reply.Code == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *client) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
	req := &protocol.DeliverRequest{GID: args.GID, NID: args.NID, CID: args.CID, UID: args.UID, Message: &protocol.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: args.Message.Buffer,
	}}
	reply := &protocol.DeliverReply{}
	err = c.client.Call(ctx, serviceDeliverMethod, req, reply)
	miss = reply.Code == code.NotFoundSession

	return
}
