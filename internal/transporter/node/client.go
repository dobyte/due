package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/internal/client"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"sync/atomic"
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

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, event cluster.Event, cid, uid int64) error {
	return c.cli.Send(ctx, protocol.EncodeTriggerReq(0, event, cid, uid))
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, cid, uid int64, message []byte) error {
	return c.cli.Send(ctx, protocol.EncodeDeliverReq(0, cid, uid, message), cid)
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
