package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/internal/code"
	"github.com/dobyte/due/transport/rpcx/internal/protocol"
	cli "github.com/smallnest/rpcx/client"
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

	c := &client{client: cli.NewXClient(
		servicePath,
		cli.Failtry,
		cli.RandomSelect,
		discovery,
		cli.DefaultOption,
	)}
	clients.Store(ep.Address(), c)

	return c, nil
}

// Trigger 触发事件
func (c *client) Trigger(ctx context.Context, event cluster.Event, gid string, uid int64) (miss bool, err error) {
	req := &protocol.TriggerRequest{Event: event, GID: gid, UID: uid}
	reply := &protocol.TriggerReply{}
	err = c.client.Call(ctx, serviceTriggerMethod, req, reply)
	miss = reply.Code == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *client) Deliver(ctx context.Context, gid, nid string, cid, uid int64, message *transport.Message) (miss bool, err error) {
	req := &protocol.DeliverRequest{GID: gid, NID: nid, CID: cid, UID: uid, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.DeliverReply{}
	err = c.client.Call(ctx, serviceDeliverMethod, req, reply)
	miss = reply.Code == code.NotFoundSession

	return
}
