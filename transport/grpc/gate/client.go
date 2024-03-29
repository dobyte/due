package gate

import (
	"context"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/session"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/grpc/code"
	"github.com/symsimmy/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"io"
)

type Client struct {
	client pb.GateClient
}

func NewClient(cc *grpc.ClientConn) *Client {
	c := &Client{client: pb.NewGateClient(cc)}
	return c
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

// IsOnline 获取指定target是否在线
func (c *Client) IsOnline(ctx context.Context, kind session.Kind, target int64) (isOnline, miss bool, err error) {
	reply, err := c.client.IsOnline(ctx, &pb.IsOnlineRequest{
		Kind:   int32(kind),
		Target: target,
	})
	if err != nil {
		miss = status.Code(err) == code.NotFoundSession
		return
	}

	isOnline = reply.IsOnline

	return
}

// GetID 获取ID
func (c *Client) GetID(ctx context.Context, kind session.Kind, target int64) (id int64, err error) {
	reply, err := c.client.GetID(ctx, &pb.GetIdRequest{
		Kind:   int32(kind),
		Target: target,
	})
	if err != nil {
		return 0, err
	}

	id = reply.Id

	return
}

// Stat 获取总在线人数
func (c *Client) Stat(ctx context.Context, kind session.Kind) (total int64, miss bool, err error) {
	reply, err := c.client.Stat(ctx, &pb.StatRequest{
		Kind: int32(kind),
	})
	if err != nil {
		miss = status.Code(err) == code.NotFoundSession
		return
	}

	total = reply.Total

	return
}

// Push 推送消息
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message *transport.Message) (miss bool, err error) {
	pushStream, err := c.client.Push(ctx, grpc.UseCompressor(gzip.Name))
	if err != nil {
		log.Errorf("get client push stream err: %v", err)
		return
	}

	err = pushStream.Send(&pb.PushRequest{
		Kind:   int32(kind),
		Target: target,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	})

	//发送也要检测EOF，当服务端在消息没接收完前主动调用SendAndClose()关闭stream，此时客户端还执行Send()，则会返回EOF错误，所以这里需要加上io.EOF判断
	if err == io.EOF {
		log.Errorf("server force close push stream err: %v", err)
	}
	if err != nil {
		log.Errorf("push stream request err: %v", err)
	}

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Multicast 推送组播消息
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *transport.Message) (int64, error) {
	multicastStream, err := c.client.Multicast(ctx, grpc.UseCompressor(gzip.Name))
	if err != nil {
		log.Errorf("get client multicast stream err: %v", err)
	}
	err = multicastStream.Send(&pb.MulticastRequest{
		Kind:    int32(kind),
		Targets: targets,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	})

	//发送也要检测EOF，当服务端在消息没接收完前主动调用SendAndClose()关闭stream，此时客户端还执行Send()，则会返回EOF错误，所以这里需要加上io.EOF判断
	if err == io.EOF {
		log.Errorf("server force close multicast stream err: %v", err)
		return 0, err
	}

	if err != nil {
		return 0, err
	}

	reply, err := multicastStream.CloseAndRecv()

	if err != nil {
		return 0, err
	}

	return reply.Total, nil
}

// Broadcast 推送广播消息
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message *transport.Message) (int64, error) {
	broadcastStream, err := c.client.Broadcast(ctx, grpc.UseCompressor(gzip.Name))
	if err != nil {
		log.Errorf("get client broadcast stream err: %v", err)
	}
	err = broadcastStream.Send(&pb.BroadcastRequest{
		Kind: int32(kind),
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	})
	//发送也要检测EOF，当服务端在消息没接收完前主动调用SendAndClose()关闭stream，此时客户端还执行Send()，则会返回EOF错误，所以这里需要加上io.EOF判断
	if err == io.EOF {
		log.Errorf("server force close broadcast stream err: %v", err)
		return 0, err
	}

	if err != nil {
		return 0, err
	}

	reply, err := broadcastStream.CloseAndRecv()

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
