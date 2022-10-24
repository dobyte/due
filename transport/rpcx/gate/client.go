package gate

import (
	"context"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/session"
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

// Bind 绑定用户与连接
func (c *client) Bind(ctx context.Context, cid, uid int64) (miss bool, err error) {
	req := &protocol.BindRequest{CID: cid, UID: uid}
	reply := &protocol.BindReply{}
	err = c.client.Call(ctx, serviceMethodBind, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// Unbind 解绑用户与连接
func (c *client) Unbind(ctx context.Context, uid int64) (miss bool, err error) {
	req := &protocol.UnbindRequest{UID: uid}
	reply := &protocol.UnbindReply{}
	err = c.client.Call(ctx, serviceMethodUnbind, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// GetIP 获取客户端IP
func (c *client) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error) {
	req := &protocol.GetIPRequest{Kind: kind, Target: target}
	reply := &protocol.GetIPReply{}
	err = c.client.Call(ctx, serviceMethodGetIP, req, reply)
	ip = reply.IP
	miss = reply.Code == code.NotFoundSession
	return
}

// Disconnect 断开连接
func (c *client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error) {
	req := &protocol.DisconnectRequest{Kind: kind, Target: target, IsForce: isForce}
	reply := &protocol.DisconnectReply{}
	err = c.client.Call(ctx, serviceMethodDisconnect, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// Push 推送消息
func (c *client) Push(ctx context.Context, kind session.Kind, target int64, message *transport.Message) (miss bool, err error) {
	req := &protocol.PushRequest{Kind: kind, Target: target, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.PushReply{}
	err = c.client.Call(ctx, serviceMethodPush, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// Multicast 推送组播消息
func (c *client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *transport.Message) (total int64, err error) {
	req := &protocol.MulticastRequest{Kind: kind, Targets: targets, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.MulticastReply{}
	err = c.client.Call(ctx, serviceMethodMulticast, req, reply)
	total = int64(reply.Total)
	return
}

// Broadcast 推送广播消息
func (c *client) Broadcast(ctx context.Context, kind session.Kind, message *transport.Message) (total int64, err error) {
	req := &protocol.BroadcastRequest{Kind: kind, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.BroadcastReply{}
	err = c.client.Call(ctx, serviceMethodBroadcast, req, reply)
	total = int64(reply.Total)
	return
}
