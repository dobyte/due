package gate

import (
	"context"
	"slices"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/bridge"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/session"
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

func (c *localClient) Bind(ctx context.Context, cid, uid int64) error {
	p := c.provider
	_, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		err := p.Bind(work, cid, uid)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	return err
}

func (c *localClient) Unbind(ctx context.Context, uid int64) error {
	p := c.provider
	_, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		err := p.Unbind(work, uid)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	return err
}

func (c *localClient) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		v, err := p.GetIP(work, kind, target)
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	if err != nil {
		return "", err
	}
	v, ok := value.(string)
	if !ok {
		return "", errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		v, err := p.Stat(work, kind)
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	if err != nil {
		return 0, err
	}
	v, ok := value.(int64)
	if !ok {
		return 0, errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		v, err := p.IsOnline(work, kind, target)
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	if err != nil {
		return false, err
	}
	v, ok := value.(bool)
	if !ok {
		return false, errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error {
	p := c.provider
	_, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		err := p.Disconnect(work, kind, target, force)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	return err
}

func (c *localClient) Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer, ack bool) error {
	buf = bridge.CopyBuffer(buf)
	p := c.provider
	_, err := c.endpoint.Submit(ctx, uint64(target), c.timeout(ack), ack, func(work context.Context) (any, error) {
		err := p.Push(work, kind, target, disconnect, buf)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, buf.Release, c.opts.WriteTimeout)
	return err
}

func (c *localClient) Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer, ack bool) (int64, error) {
	if len(targets) > 1<<16-1 {
		buf.Release()
		return 0, errors.ErrInvalidArgument
	}
	targets = slices.Clone(targets)
	buf = bridge.CopyBuffer(buf)
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(ack), ack, func(work context.Context) (any, error) {
		v, err := p.Multicast(work, kind, targets, disconnect, buf)
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, buf.Release, c.opts.WriteTimeout)
	if err != nil {
		return 0, err
	}
	if !ack {
		return 0, nil
	}
	v, ok := value.(int64)
	if !ok {
		return 0, errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer, ack bool) (int64, error) {
	buf = bridge.CopyBuffer(buf)
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(ack), ack, func(work context.Context) (any, error) {
		v, err := p.Broadcast(work, kind, disconnect, buf)
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, buf.Release, c.opts.WriteTimeout)
	if err != nil {
		return 0, err
	}
	if !ack {
		return 0, nil
	}
	v, ok := value.(int64)
	if !ok {
		return 0, errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer, ack bool) (int64, error) {
	if len(channel) > 1<<8-1 {
		buf.Release()
		return 0, errors.ErrInvalidArgument
	}
	buf = bridge.CopyBuffer(buf)
	p := c.provider
	value, err := c.endpoint.Submit(ctx, 0, c.timeout(ack), ack, func(work context.Context) (any, error) {
		v, err := p.Publish(work, channel, disconnect, buf)
		return v, codes.CodeToError(codes.ErrorToCode(err))
	}, buf.Release, c.opts.WriteTimeout)
	if err != nil {
		return 0, err
	}
	if !ack {
		return 0, nil
	}
	v, ok := value.(int64)
	if !ok {
		return 0, errors.ErrInvalidArgument
	}
	return v, nil
}

func (c *localClient) Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if len(targets) > 1<<16-1 || len(channel) > 1<<8-1 {
		return errors.ErrInvalidArgument
	}
	targets = slices.Clone(targets)
	p := c.provider
	_, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		err := p.Subscribe(work, kind, targets, channel)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
	return err
}

func (c *localClient) Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error {
	if len(targets) > 1<<16-1 || len(channel) > 1<<8-1 {
		return errors.ErrInvalidArgument
	}
	targets = slices.Clone(targets)
	p := c.provider
	_, err := c.endpoint.Submit(ctx, 0, c.timeout(true), true, func(work context.Context) (any, error) {
		err := p.Unsubscribe(work, kind, targets, channel)
		return nil, codes.CodeToError(codes.ErrorToCode(err))
	}, nil, c.opts.WriteTimeout)
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
