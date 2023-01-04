package gate

import (
	"context"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/session"
	"github.com/dobyte/due/transport"
	innerclient "github.com/dobyte/due/transport/grpc/internal/client"
	"github.com/dobyte/due/transport/grpc/internal/code"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"sync"
)

var clients sync.Map

type client struct {
	client pb.GateClient
}

func NewClient(ep *router.Endpoint, opts *innerclient.Options) (*client, error) {
	cli, ok := clients.Load(ep.Address())
	if ok {
		return cli.(*client), nil
	}

	opts.Addr = ep.Address()
	opts.IsSecure = ep.IsSecure()

	conn, err := innerclient.Dial(opts)
	if err != nil {
		return nil, err
	}

	cc := &client{client: pb.NewGateClient(conn)}
	clients.Store(ep.Address(), cc)

	return cc, nil
}

// Bind 绑定用户与连接
func (c *client) Bind(ctx context.Context, cid, uid int64) (miss bool, err error) {
	_, err = c.client.Bind(ctx, &pb.BindRequest{
		CID: cid,
		UID: uid,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Unbind 解绑用户与连接
func (c *client) Unbind(ctx context.Context, uid int64) (miss bool, err error) {
	_, err = c.client.Unbind(ctx, &pb.UnbindRequest{
		UID: uid,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// GetIP 获取客户端IP
func (c *client) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error) {
	reply, err := c.client.GetIP(ctx, &pb.GetIPRequest{
		Kind:   int32(kind),
		Target: target,
	})
	if err != nil {
		miss = status.Code(err) == code.NotFoundSession
		return
	}

	ip = reply.IP

	return
}

// Push 推送消息
func (c *client) Push(ctx context.Context, kind session.Kind, target int64, message *transport.Message) (miss bool, err error) {
	_, err = c.client.Push(ctx, &pb.PushRequest{
		Kind:   int32(kind),
		Target: target,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}, grpc.UseCompressor(gzip.Name))

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Multicast 推送组播消息
func (c *client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *transport.Message) (int64, error) {
	reply, err := c.client.Multicast(ctx, &pb.MulticastRequest{
		Kind:    int32(kind),
		Targets: targets,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}, grpc.UseCompressor(gzip.Name))
	if err != nil {
		return 0, err
	}

	return reply.Total, nil
}

// Broadcast 推送广播消息
func (c *client) Broadcast(ctx context.Context, kind session.Kind, message *transport.Message) (int64, error) {
	reply, err := c.client.Broadcast(ctx, &pb.BroadcastRequest{
		Kind: int32(kind),
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}, grpc.UseCompressor(gzip.Name))
	if err != nil {
		return 0, err
	}

	return reply.Total, nil
}

// Disconnect 断开连接
func (c *client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error) {
	_, err = c.client.Disconnect(ctx, &pb.DisconnectRequest{
		Kind:    int32(kind),
		Target:  target,
		IsForce: isForce,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}
