package gate

import (
	"context"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/session"
)

type Client struct {
	seq atomic.Uint64
	cli *drpc.Client
}

func NewClient(cli *drpc.Client) *Client {
	return &Client{
		cli: cli,
	}
}

// Bind binds the connection cid and user uid together.
func (c *Client) Bind(ctx context.Context, cid, uid int64) error {
	seq := c.doGenSequence()
	req := protocol.EncodeBindReq(seq, cid, uid)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeBindRes(res)

	res.Release()

	if err != nil {
		return err
	} else {
		return codes.CodeToError(code)
	}
}

// Unbind unbinds the user from its connection.
func (c *Client) Unbind(ctx context.Context, uid int64) error {
	seq := c.doGenSequence()
	req := protocol.EncodeUnbindReq(seq, uid)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeUnbindRes(res)

	res.Release()

	if err != nil {
		return err
	} else {
		return codes.CodeToError(code)
	}
}

// GetIP returns the client IP of the target session.
func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	seq := c.doGenSequence()
	req := protocol.EncodeGetIPReq(seq, kind, target)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return "", err
	}

	code, ip, err := protocol.DecodeGetIPRes(res)

	res.Release()

	if err != nil {
		return "", err
	} else {
		return ip, codes.CodeToError(code)
	}
}

// Stat returns the total number of sessions of the given kind.
func (c *Client) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	seq := c.doGenSequence()
	req := protocol.EncodeStatReq(seq, kind)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return 0, err
	}

	code, total, err := protocol.DecodeStatRes(res)

	res.Release()

	if err != nil {
		return 0, err
	} else {
		return int64(total), codes.CodeToError(code)
	}
}

// IsOnline reports whether the target session is online.
func (c *Client) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	seq := c.doGenSequence()
	req := protocol.EncodeIsOnlineReq(seq, kind, target)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return false, err
	}

	code, isOnline, err := protocol.DecodeIsOnlineRes(res)

	res.Release()

	if err != nil {
		return false, err
	} else {
		return isOnline, codes.CodeToError(code)
	}
}

// Disconnect disconnects the target session.
func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	seq := c.doGenSequence()
	req := protocol.EncodeDisconnectReq(seq, kind, target, force)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeDisconnectRes(res)

	res.Release()

	if err != nil {
		return err
	} else {
		return codes.CodeToError(code)
	}
}

// Push pushes a message to the target. When ack is true it waits for the response.
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer, ack bool) error {
	if ack {
		seq := c.doGenSequence()
		req := protocol.EncodePushReq(seq, kind, target, disconnect, buf)

		res, err := c.cli.Call(ctx, seq, req)
		if err != nil {
			return err
		}

		code, err := protocol.DecodePushRes(res)

		res.Release()

		if err != nil {
			return err
		} else {
			return codes.CodeToError(code)
		}
	} else {
		return c.cli.Push(ctx, protocol.EncodePushReq(0, kind, target, disconnect, buf), target)
	}
}

// Multicast pushes a message to multiple targets. When ack is true it waits for the response and
// returns the number of targets that received the message.
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer, ack bool) (int64, error) {
	if len(targets) > 1<<16-1 {
		buf.Release()
		return 0, errors.ErrInvalidArgument
	}

	if ack {
		seq := c.doGenSequence()
		req := protocol.EncodeMulticastReq(seq, kind, targets, disconnect, buf)

		res, err := c.cli.Call(ctx, seq, req)
		if err != nil {
			return 0, err
		}

		code, total, err := protocol.DecodeMulticastRes(res)

		res.Release()

		if err != nil {
			return 0, err
		} else {
			return int64(total), codes.CodeToError(code)
		}
	} else {
		return 0, c.cli.Push(ctx, protocol.EncodeMulticastReq(0, kind, targets, disconnect, buf))
	}
}

// Broadcast pushes a message to all sessions. When ack is true it waits for the response and
// returns the number of sessions that received the message.
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer, ack bool) (int64, error) {
	if ack {
		seq := c.doGenSequence()
		req := protocol.EncodeBroadcastReq(seq, kind, disconnect, buf)

		res, err := c.cli.Call(ctx, seq, req)
		if err != nil {
			return 0, err
		}

		code, total, err := protocol.DecodeBroadcastRes(res)

		res.Release()

		if err != nil {
			return 0, err
		} else {
			return int64(total), codes.CodeToError(code)
		}
	} else {
		return 0, c.cli.Push(ctx, protocol.EncodeBroadcastReq(0, kind, disconnect, buf))
	}
}

// Publish publishes a message to a channel. When ack is true it waits for the response and returns
// the number of sessions that received the message.
func (c *Client) Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer, ack bool) (int64, error) {
	if len(channel) > 1<<8-1 {
		buf.Release()
		return 0, errors.ErrInvalidArgument
	}

	if ack {
		seq := c.doGenSequence()
		req := protocol.EncodePublishReq(seq, channel, disconnect, buf)

		res, err := c.cli.Call(ctx, seq, req)
		if err != nil {
			return 0, err
		}

		code, total, err := protocol.DecodePublishRes(res)

		res.Release()

		if err != nil {
			return 0, err
		} else {
			return int64(total), codes.CodeToError(code)
		}
	} else {
		return 0, c.cli.Push(ctx, protocol.EncodePublishReq(0, channel, disconnect, buf))
	}
}

// Subscribe subscribes the targets to a channel.
func (c *Client) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if len(targets) > 1<<16-1 {
		return errors.ErrInvalidArgument
	}

	if len(channel) > 1<<8-1 {
		return errors.ErrInvalidArgument
	}

	seq := c.doGenSequence()
	req := protocol.EncodeSubscribeReq(seq, kind, targets, channel)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeSubscribeRes(res)

	res.Release()

	if err != nil {
		return err
	} else {
		return codes.CodeToError(code)
	}
}

// Unsubscribe unsubscribes the targets from a channel.
func (c *Client) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if len(targets) > 1<<16-1 {
		return errors.ErrInvalidArgument
	}

	if len(channel) > 1<<8-1 {
		return errors.ErrInvalidArgument
	}

	seq := c.doGenSequence()
	req := protocol.EncodeUnsubscribeReq(seq, kind, targets, channel)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeUnsubscribeRes(res)

	res.Release()

	if err != nil {
		return err
	} else {
		return codes.CodeToError(code)
	}
}

// GetState returns the state of the gate.
func (c *Client) GetState(ctx context.Context) (cluster.State, error) {
	seq := c.doGenSequence()
	req := protocol.EncodeGetStateReq(seq)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return 0, err
	}

	code, state, err := protocol.DecodeGetStateRes(res)

	res.Release()

	if err != nil {
		return 0, err
	} else {
		return state, codes.CodeToError(code)
	}
}

// SetState sets the state of the gate.
func (c *Client) SetState(ctx context.Context, state cluster.State) error {
	seq := c.doGenSequence()
	req := protocol.EncodeSetStateReq(seq, state)

	res, err := c.cli.Call(ctx, seq, req)
	if err != nil {
		return err
	}

	code, err := protocol.DecodeSetStateRes(res)

	res.Release()

	if err != nil {
		return err
	} else {
		return codes.CodeToError(code)
	}
}

// doGenSequence generates a sequence number, skipping the value 0.
func (c *Client) doGenSequence() (seq uint64) {
	if seq := c.seq.Add(1); seq == 0 {
		return c.seq.Add(1)
	} else {
		return seq
	}
}
