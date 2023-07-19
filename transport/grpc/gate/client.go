package gate

import (
	"context"
	"github.com/dobyte/due/transport/grpc/v2/internal/code"
	"github.com/dobyte/due/transport/grpc/v2/internal/pb"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

type Client struct {
	client pb.GateClient
}

func NewClient(cc *grpc.ClientConn) *Client {
	return &Client{client: pb.NewGateClient(cc)}
}

// Bind 绑定用户与连接
func (c *Client) Bind(ctx context.Context, cid, uid int64) (miss bool, err error) {
	_, err = c.client.Bind(ctx, &pb.BindRequest{
		CID: cid,
		UID: uid,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Unbind 解绑用户与连接
func (c *Client) Unbind(ctx context.Context, uid int64) (miss bool, err error) {
	_, err = c.client.Unbind(ctx, &pb.UnbindRequest{
		UID: uid,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// GetIP 获取客户端IP
func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error) {
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
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message *packet.Message) (miss bool, err error) {
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
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *packet.Message) (int64, error) {
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
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message *packet.Message) (int64, error) {
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

// Stat 统计会话总数
func (c *Client) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	reply, err := c.client.Stat(ctx, &pb.StatRequest{
		Kind: int32(kind),
	})
	if err != nil {
		return 0, err
	}

	return reply.Total, nil
}

// Disconnect 断开连接
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error) {
	_, err = c.client.Disconnect(ctx, &pb.DisconnectRequest{
		Kind:    int32(kind),
		Target:  target,
		IsForce: isForce,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}
