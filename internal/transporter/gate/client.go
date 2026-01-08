package gate

import (
	"context"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
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
func (c *Client) Bind(ctx context.Context, cid, uid int64) error {
	seq := c.doGenSequence()
	buf := protocol.EncodeBindReq(seq, cid, uid)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}
	defer res.Release()

	code, err := protocol.DecodeBindRes(res.Bytes())
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// Unbind 解绑用户与连接
func (c *Client) Unbind(ctx context.Context, uid int64) error {
	seq := c.doGenSequence()
	buf := protocol.EncodeUnbindReq(seq, uid)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}
	defer res.Release()

	code, err := protocol.DecodeUnbindRes(res.Bytes())
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// GetIP 获取客户端IP
func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	seq := c.doGenSequence()
	buf := protocol.EncodeGetIPReq(seq, kind, target)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return "", err
	}
	defer res.Release()

	code, ip, err := protocol.DecodeGetIPRes(res.Bytes())
	if err != nil {
		return "", err
	}

	return ip, codes.CodeToError(code)
}

// Stat 推送广播消息
func (c *Client) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	seq := c.doGenSequence()
	buf := protocol.EncodeStatReq(seq, kind)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return 0, err
	}
	defer res.Release()

	code, total, err := protocol.DecodeStatRes(res.Bytes())
	if err != nil {
		return 0, err
	}

	return int64(total), codes.CodeToError(code)
}

// IsOnline 检测是否在线
func (c *Client) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	seq := c.doGenSequence()
	buf := protocol.EncodeIsOnlineReq(seq, kind, target)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return false, err
	}
	defer res.Release()

	code, isOnline, err := protocol.DecodeIsOnlineRes(res.Bytes())
	if err != nil {
		return false, err
	}

	return isOnline, codes.CodeToError(code)
}

// Disconnect 断开连接
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	seq := c.doGenSequence()
	buf := protocol.EncodeDisconnectReq(seq, kind, target, force)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}
	defer res.Release()

	code, err := protocol.DecodeDisconnectRes(res.Bytes())
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// Push 推送消息
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message buffer.Buffer, ack ...bool) error {
	if len(ack) > 0 && ack[0] {
		seq := c.doGenSequence()
		buf := protocol.EncodePushReq(seq, kind, target, message)

		res, err := c.cli.Call(ctx, seq, buf)
		if err != nil {
			return err
		}
		defer res.Release()

		code, err := protocol.DecodePushRes(res.Bytes())
		if err != nil {
			return err
		}

		return codes.CodeToError(code)
	} else {
		return c.cli.Send(ctx, protocol.EncodePushReq(0, kind, target, message), target)
	}
}

// Multicast 推送组播消息
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message buffer.Buffer, ack ...bool) (int64, error) {
	if len(ack) > 0 && ack[0] {
		seq := c.doGenSequence()
		buf := protocol.EncodeMulticastReq(seq, kind, targets, message)

		res, err := c.cli.Call(ctx, seq, buf)
		if err != nil {
			return 0, err
		}
		defer res.Release()

		code, total, err := protocol.DecodeMulticastRes(res.Bytes())
		if err != nil {
			return 0, err
		}

		return int64(total), codes.CodeToError(code)
	} else {
		return 0, c.cli.Send(ctx, protocol.EncodeMulticastReq(0, kind, targets, message))
	}
}

// Broadcast 推送广播消息
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message buffer.Buffer, ack ...bool) (int64, error) {
	if len(ack) > 0 && ack[0] {
		seq := c.doGenSequence()
		buf := protocol.EncodeBroadcastReq(seq, kind, message)

		res, err := c.cli.Call(ctx, seq, buf)
		if err != nil {
			return 0, err
		}
		defer res.Release()

		code, total, err := protocol.DecodeBroadcastRes(res.Bytes())
		if err != nil {
			return 0, err
		}

		return int64(total), codes.CodeToError(code)
	} else {
		return 0, c.cli.Send(ctx, protocol.EncodeBroadcastReq(0, kind, message))
	}
}

// Publish 发布频道消息
func (c *Client) Publish(ctx context.Context, channel string, message buffer.Buffer, ack ...bool) (int64, error) {
	if len(channel) > 1<<8-1 {
		message.Release()
		return 0, errors.ErrInvalidArgument
	}

	if len(ack) > 0 && ack[0] {
		seq := c.doGenSequence()
		buf := protocol.EncodePublishReq(seq, channel, message)

		res, err := c.cli.Call(ctx, seq, buf)
		if err != nil {
			return 0, err
		}
		defer res.Release()

		total, err := protocol.DecodePublishRes(res.Bytes())
		if err != nil {
			return 0, err
		}

		return int64(total), nil
	} else {
		return 0, c.cli.Send(ctx, protocol.EncodePublishReq(0, channel, message))
	}
}

// Subscribe 订阅频道
func (c *Client) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if len(channel) > 1<<8-1 {
		return errors.ErrInvalidArgument
	}

	seq := c.doGenSequence()
	buf := protocol.EncodeSubscribeReq(seq, kind, targets, channel)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}
	defer res.Release()

	code, err := protocol.DecodeSubscribeRes(res.Bytes())
	if err != nil {
		return err
	}

	return codes.CodeToError(code)
}

// Unsubscribe 取消订阅频道
func (c *Client) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if len(channel) > 1<<8-1 {
		return errors.ErrInvalidArgument
	}

	seq := c.doGenSequence()
	buf := protocol.EncodeUnsubscribeReq(seq, kind, targets, channel)

	res, err := c.cli.Call(ctx, seq, buf)
	if err != nil {
		return err
	}
	defer res.Release()

	code, err := protocol.DecodeUnsubscribeRes(res.Bytes())
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
	defer res.Release()

	code, state, err := protocol.DecodeGetStateRes(res.Bytes())
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
	defer res.Release()

	code, err := protocol.DecodeSetStateRes(res.Bytes())
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
