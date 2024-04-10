package gate

import (
	"context"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/dobyte/due/transport/kitex/v2/internal/protocol/gate"
	inner "github.com/dobyte/due/transport/kitex/v2/internal/protocol/gate/gate"
	"github.com/dobyte/due/transport/kitex/v2/internal/protocol/message"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"time"
)

type Client struct {
	cli inner.Client
}

func NewClient(addr string) (*Client, error) {
	cli, err := inner.NewClient("Gate", client.WithHostPorts(addr), client.WithLongConnection(connpool.IdleConfig{
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

// Bind 绑定用户与连接
func (c *Client) Bind(ctx context.Context, cid, uid int64) (miss bool, err error) {
	_, err = c.cli.Bind(ctx, &gate.BindRequest{
		CID: cid,
		UID: uid,
	})
	return
}

// Unbind 解绑用户与连接
func (c *Client) Unbind(ctx context.Context, uid int64) (miss bool, err error) {
	_, err = c.cli.Unbind(ctx, &gate.UnbindRequest{
		UID: uid,
	})
	return
}

// GetIP 获取客户端IP
func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error) {
	var resp *gate.GetIPResponse

	resp, err = c.cli.GetIP(ctx, &gate.GetIPRequest{
		Kind:   int8(kind),
		Target: target,
	})
	if err != nil {
		return
	}

	ip = resp.IP

	return
}

// Push 推送消息
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, msg *packet.Message) (miss bool, err error) {
	_, err = c.cli.Push(ctx, &gate.PushRequest{
		Kind:   int8(kind),
		Target: target,
		Message: &message.Message{
			Seq:    msg.Seq,
			Route:  msg.Route,
			Buffer: msg.Buffer,
		},
	})
	return
}

// Multicast 推送组播消息
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, msg *packet.Message) (total int64, err error) {
	var resp *gate.MulticastResponse

	resp, err = c.cli.Multicast(ctx, &gate.MulticastRequest{
		Kind:    int8(kind),
		Targets: targets,
		Message: &message.Message{
			Seq:    msg.Seq,
			Route:  msg.Route,
			Buffer: msg.Buffer,
		},
	})
	if err != nil {
		return
	}

	total = resp.Total

	return
}

// Broadcast 推送广播消息
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, msg *packet.Message) (total int64, err error) {
	var resp *gate.BroadcastResponse

	resp, err = c.cli.Broadcast(ctx, &gate.BroadcastRequest{
		Kind: int8(kind),
		Message: &message.Message{
			Seq:    msg.Seq,
			Route:  msg.Route,
			Buffer: msg.Buffer,
		},
	})
	if err != nil {
		return
	}

	total = resp.Total

	return
}

// Stat 统计会话总数
func (c *Client) Stat(ctx context.Context, kind session.Kind) (total int64, err error) {
	var resp *gate.StatResponse

	resp, err = c.cli.Stat(ctx, &gate.StatRequest{
		Kind: int8(kind),
	})
	if err != nil {
		return
	}

	total = resp.Total

	return
}

// Disconnect 断开连接
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error) {
	_, err = c.cli.Disconnect(ctx, &gate.DisconnectRequest{
		Kind:    int8(kind),
		Target:  target,
		IsForce: isForce,
	})
	return
}
