package gate

import (
	"context"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/internal/stream/internal/client"
	"github.com/dobyte/due/v2/internal/stream/internal/codes"
	"github.com/dobyte/due/v2/internal/stream/internal/protocol"
	"github.com/dobyte/due/v2/session"
	"sync"
	"sync/atomic"
)

type Client struct {
	seq      uint64
	channels sync.Map // 通道

	client *client.Client
}

func NewClient(ep *endpoint.Endpoint) *Client {
	c := &Client{}

}

func (c *Client) doGenSeq() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

// Bind 绑定用户与连接
func (c *Client) Bind(ctx context.Context, cid, uid int64) (bool, error) {
	seq := c.doGenSeq()

	buf := protocol.EncodeBindReq(seq, cid, uid)

	res, err := c.client.Call(ctx, seq, buf)
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
	seq := c.doGenSeq()

	buf := protocol.EncodeUnbindReq(seq, uid)

	res, err := c.client.Call(ctx, seq, buf)
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
	seq := c.doGenSeq()

	buf := protocol.EncodeGetIPReq(seq, kind, target)

	res, err := c.client.Call(ctx, seq, buf)
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
	seq := c.doGenSeq()

	buf := protocol.EncodeStatReq(seq, kind)

	res, err := c.client.Call(ctx, seq, buf)
	if err != nil {
		return 0, err
	}

	_, total, err := protocol.DecodeStatRes(res)

	return int64(total), err
}

// Disconnect 断开连接
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (bool, error) {
	seq := c.doGenSeq()

	buf := protocol.EncodeDisconnectReq(seq, kind, target, isForce)

	res, err := c.client.Call(ctx, seq, buf)
	if err != nil {
		return false, err
	}

	code, err := protocol.DecodeDisconnectRes(res)
	if err != nil {
		return false, err
	}

	return code == codes.NotFoundSession, nil
}

// Push 推送消息
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message []byte) (bool, error) {
	seq := c.doGenSeq()

	buf := protocol.EncodePushReq(seq, kind, target, message)

	res, err := c.client.Call(ctx, seq, buf, target)
	if err != nil {
		return false, err
	}

	code, err := protocol.DecodePushRes(res)
	if err != nil {
		return false, err
	}

	return code == codes.NotFoundSession, nil
}

// AsyncPush 异步推送消息
func (c *Client) AsyncPush(ctx context.Context, kind session.Kind, target int64, message []byte) error {
	buf := protocol.EncodePushReq(0, kind, target, message)

	return c.client.Send(ctx, buf, target)
}

// Multicast 推送组播消息
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message []byte) (int64, error) {
	seq := c.doGenSeq()

	buf := protocol.EncodeMulticastReq(seq, kind, targets, message)

	res, err := c.client.Call(ctx, seq, buf)
	if err != nil {
		return 0, err
	}

	_, total, err := protocol.DecodeMulticastRes(res)
	if err != nil {
		return 0, err
	}

	return int64(total), nil
}

// AsyncMulticast 推送组播消息
func (c *Client) AsyncMulticast(ctx context.Context, kind session.Kind, targets []int64, message []byte) error {
	buf := protocol.EncodeMulticastReq(0, kind, targets, message)

	return c.client.Send(ctx, buf)
}

// Broadcast 推送广播消息
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message []byte) (int64, error) {
	seq := c.doGenSeq()

	buf := protocol.EncodeBroadcastReq(seq, kind, message)

	res, err := c.client.Call(ctx, seq, buf)
	if err != nil {
		return 0, err
	}

	_, total, err := protocol.DecodeBroadcastRes(res)
	if err != nil {
		return 0, err
	}

	return int64(total), nil
}

// AsyncBroadcast 推送广播消息
func (c *Client) AsyncBroadcast(ctx context.Context, kind session.Kind, message []byte) error {
	buf := protocol.EncodeBroadcastReq(0, kind, message)

	return c.client.Send(ctx, buf)
}
