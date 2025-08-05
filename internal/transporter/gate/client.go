package gate

import (
	"context"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/client"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

type Client struct {
	seq uint64
	cli *client.Client
}

func NewClient(cli *client.Client) *Client {
	return &Client{
		cli: cli,
	}
}

// Bind 绑定用户与连接
func (c *Client) Bind(ctx context.Context, cid, uid int64) (bool, error) {
	seq := c.doGenSequence()

	buf := protocol.EncodeBindReq(seq, cid, uid)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return false, err
	}

	code, err := protocol.DecodeBindRes(res)
	if err != nil {
		return false, err
	}

	return code == codes.NotFoundSession, nil
}

// Unbind 解绑用户与连接
func (c *Client) Unbind(ctx context.Context, uid int64) (bool, error) {
	seq := c.doGenSequence()

	buf := protocol.EncodeUnbindReq(seq, uid)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return false, err
	}

	code, err := protocol.DecodeUnbindRes(res)
	if err != nil {
		return false, err
	}

	return code == codes.NotFoundSession, nil
}

// GetIP 获取客户端IP
func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (string, bool, error) {
	seq := c.doGenSequence()

	buf := protocol.EncodeGetIPReq(seq, kind, target)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return "", false, err
	}

	code, ip, err := protocol.DecodeGetIPRes(res)
	if err != nil {
		return "", false, err
	}

	return ip, code == codes.NotFoundSession, nil
}

// Stat 推送广播消息
func (c *Client) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	seq := c.doGenSequence()

	buf := protocol.EncodeStatReq(seq, kind)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return 0, err
	}

	_, total, err := protocol.DecodeStatRes(res)

	return int64(total), err
}

// IsOnline 检测是否在线
func (c *Client) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, bool, error) {
	seq := c.doGenSequence()

	buf := protocol.EncodeIsOnlineReq(seq, kind, target)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return false, false, err
	}

	code, isOnline, err := protocol.DecodeIsOnlineRes(res)
	if err != nil {
		return false, false, err
	}

	return code == codes.NotFoundSession, isOnline, nil
}

// Disconnect 断开连接
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	if force {
		return c.cli.Send(ctx, protocol.EncodeDisconnectReq(0, kind, target, force))
	} else {
		return c.cli.Send(ctx, protocol.EncodeDisconnectReq(0, kind, target, force), target)
	}
}

// Push 推送消息（异步）
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message buffer.Buffer) error {
	return c.cli.Send(ctx, protocol.EncodePushReq(0, kind, target, message), target)
}

// Multicast 推送组播消息（异步）
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message buffer.Buffer) error {
	return c.cli.Send(ctx, protocol.EncodeMulticastReq(0, kind, targets, message))
}

// Broadcast 推送广播消息（异步）
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message buffer.Buffer) error {
	return c.cli.Send(ctx, protocol.EncodeBroadcastReq(0, kind, message))
}

// Publish 发布频道消息（异步）
func (c *Client) Publish(ctx context.Context, channel string, message buffer.Buffer) error {
	return c.cli.Send(ctx, protocol.EncodePublishReq(0, channel, message))
}

// Subscribe 订阅频道
func (c *Client) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	seq := c.doGenSequence()

	buf := protocol.EncodeSubscribeReq(seq, kind, targets, channel)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeSubscribeRes(res)
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// Unsubscribe 取消订阅频道
func (c *Client) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	seq := c.doGenSequence()

	buf := protocol.EncodeUnsubscribeReq(seq, kind, targets, channel)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeUnsubscribeRes(res)
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// GetState 获取状态
func (c *Client) GetState(ctx context.Context) (cluster.State, error) {
	seq := c.doGenSequence()

	buf := protocol.EncodeGetStateReq(seq)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return 0, err
	}

	code, state, err := protocol.DecodeGetStateRes(res)
	if err != nil {
		return 0, err
	}

	return state, codes.CodeToError(code)
}

// SetState 设置状态
func (c *Client) SetState(ctx context.Context, state cluster.State) error {
	seq := c.doGenSequence()

	buf := protocol.EncodeSetStateReq(seq, state)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeSetStateRes(res)
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// 生成序列号，规避生成序列号为0的编号
func (c *Client) doGenSequence() (seq uint64) {
	for {
		if seq = atomic.AddUint64(&c.seq, 1); seq != 0 {
			return
		}
	}
}
