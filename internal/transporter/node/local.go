package node

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/bridge"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
)

type localClient struct {
	endpoint *bridge.Endpoint
	provider Provider
	opts     ClientOptions
}

func (c *localClient) timeout(ack bool) time.Duration {
	if ack {
		return c.opts.CallTimeout
	}
	return c.opts.WriteTimeout
}

func (c *localClient) Trigger(ctx context.Context, event cluster.Event, cid, uid int64) error {
	if c.opts.Kind != cluster.Gate {
		return errors.ErrIllegalRequest
	}
	gid := c.opts.ID
	p := c.provider
	_, err := c.endpoint.Submit(ctx, uint64(cid), c.timeout(false), false, func(work context.Context) (any, error) {
		err := p.Trigger(work, gid, cid, uid, event)
		if errors.Is(err, errors.ErrNotFoundSession) {
			err = nil
		}
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	return err
}

func (c *localClient) Deliver(ctx context.Context, cid, uid int64, buf buffer.Buffer) error {
	var gid, nid string
	switch c.opts.Kind {
	case cluster.Gate:
		gid = c.opts.ID
	case cluster.Node:
		nid = c.opts.ID
	default:
		buf.Release()
		return errors.ErrIllegalRequest
	}
	buf = bridge.CopyBuffer(buf)
	p := c.provider
	_, err := c.endpoint.Submit(ctx, uint64(cid), c.timeout(false), false, func(work context.Context) (any, error) {
		err := p.Deliver(work, gid, nid, cid, uid, buf)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, buf.Release, c.opts.WriteTimeout)
	return err
}

func (c *localClient) GetState(ctx context.Context) (cluster.State, error) {
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		v, err := p.GetState()
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	if err != nil {
		return 0, err
	}
	v, ok := value.(cluster.State)
	if !ok {
		return 0, errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) SetState(ctx context.Context, state cluster.State) error {
	p := c.provider
	_, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		err := p.SetState(state)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	return err
}
