package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"golang.org/x/sync/singleflight"
)

type ClientOptions = drpc.ClientOptions

type Builder struct {
	sfg     singleflight.Group
	opts    *ClientOptions
	clients sync.Map
}

func NewBuilder(opts *ClientOptions) *Builder {
	return &Builder{
		opts: opts,
	}
}

// Build returns a client for addr, reusing a cached client when one already exists.
//
// Dialing is bounded by the dial timeout so that an unreachable endpoint does not trigger
// indefinite retries.
func (b *Builder) Build(addr string) (*Client, error) {
	if cli, ok := b.clients.Load(addr); ok {
		return cli.(*Client), nil
	}

	cli, err, _ := b.sfg.Do(addr, func() (any, error) {
		if cli, ok := b.clients.Load(addr); ok {
			return cli.(*Client), nil
		}

		c, err := drpc.NewClient(addr, b.opts)
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), b.opts.DialTimeout)
		defer cancel()

		if err = c.Establish(ctx); err != nil {
			_ = c.Close()

			return nil, err
		}

		cli := NewClient(c)

		b.clients.Store(addr, cli)

		return cli, nil
	})
	if err != nil {
		return nil, err
	}

	return cli.(*Client), nil
}
