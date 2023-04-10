package gate

import (
	"context"
	"github.com/dobyte/due/session"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/internal/code"
	"github.com/dobyte/due/transport/rpcx/internal/protocol"
	cli "github.com/smallnest/rpcx/client"
)

type Client struct {
	cli *cli.OneClient
}

func NewClient(cli *cli.OneClient) *Client {
	return &Client{cli: cli}
}

// Bind 绑定用户与连接
func (c *Client) Bind(ctx context.Context, cid, uid int64) (miss bool, err error) {
	req := &protocol.BindRequest{CID: cid, UID: uid}
	reply := &protocol.BindReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodBind, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// Unbind 解绑用户与连接
func (c *Client) Unbind(ctx context.Context, uid int64) (miss bool, err error) {
	req := &protocol.UnbindRequest{UID: uid}
	reply := &protocol.UnbindReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodUnbind, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// GetIP 获取客户端IP
func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error) {
	req := &protocol.GetIPRequest{Kind: kind, Target: target}
	reply := &protocol.GetIPReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodGetIP, req, reply)
	ip = reply.IP
	miss = reply.Code == code.NotFoundSession
	return
}

// Disconnect 断开连接
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error) {
	req := &protocol.DisconnectRequest{Kind: kind, Target: target, IsForce: isForce}
	reply := &protocol.DisconnectReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodDisconnect, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// Push 推送消息
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message *transport.Message) (miss bool, err error) {
	req := &protocol.PushRequest{Kind: kind, Target: target, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.PushReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodPush, req, reply)
	miss = reply.Code == code.NotFoundSession
	return
}

// Multicast 推送组播消息
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *transport.Message) (total int64, err error) {
	req := &protocol.MulticastRequest{Kind: kind, Targets: targets, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.MulticastReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodMulticast, req, reply)
	total = reply.Total
	return
}

// Broadcast 推送广播消息
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message *transport.Message) (total int64, err error) {
	req := &protocol.BroadcastRequest{Kind: kind, Message: &protocol.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: message.Buffer,
	}}
	reply := &protocol.BroadcastReply{}
	err = c.cli.Call(ctx, ServicePath, serviceMethodBroadcast, req, reply)
	total = reply.Total
	return
}
