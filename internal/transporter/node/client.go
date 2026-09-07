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

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, event cluster.Event, cid, uid int64) error {
	return c.cli.Send(ctx, protocol.EncodeTriggerReq(0, event, cid, uid), cid)
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, cid, uid int64, buf buffer.Buffer) error {
	return c.cli.Send(ctx, protocol.EncodeDeliverReq(0, cid, uid, buf), cid)
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
func (c *Client) doGenSequence() uint64 {
	if seq := c.seq.Add(1); seq == 0 {
		return c.seq.Add(1)
	} else {
		return seq
	}
}
