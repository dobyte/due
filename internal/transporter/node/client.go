package node

import (
	"context"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
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

// Trigger triggers an event on the node.
func (c *Client) Trigger(ctx context.Context, event cluster.Event, cid, uid int64) error {
	return c.cli.Push(ctx, protocol.EncodeTriggerReq(0, event, cid, uid), cid)
}

// Deliver delivers a message to the node.
func (c *Client) Deliver(ctx context.Context, cid, uid int64, buf buffer.Buffer) error {
	return c.cli.Push(ctx, protocol.EncodeDeliverReq(0, cid, uid, buf), cid)
}

// GetState returns the state of the node.
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

// SetState sets the state of the node.
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
func (c *Client) doGenSequence() uint64 {
	if seq := c.seq.Add(1); seq == 0 {
		return c.seq.Add(1)
	} else {
		return seq
	}
}
