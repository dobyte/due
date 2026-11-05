package gate

import (
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"golang.org/x/sync/singleflight"
)

type ClientOptions = drpc.ClientOptions

type Builder struct {
	sfg     singleflight.Group
	opts    *ClientOptions
	clients sync.Map
	closed  atomic.Bool
}

func NewBuilder(opts *ClientOptions) *Builder {
	return &Builder{
		opts: opts,
	}
}

// Build returns a client for addr, reusing a cached client when one already exists.
//
// It returns [errors.ErrClientClosed] once the builder has been closed.
func (b *Builder) Build(addr string) (*Client, error) {
	if b.closed.Load() {
		return nil, errors.ErrClientClosed
	}

	if cli, ok := b.clients.Load(addr); ok {
		return cli.(*Client), nil
	}

	cli, err, _ := b.sfg.Do(addr, func() (any, error) {
		if b.closed.Load() {
			return nil, errors.ErrClientClosed
		}

		if cli, ok := b.clients.Load(addr); ok {
			return cli.(*Client), nil
		}

		c, err := drpc.NewClient(addr, b.opts)
		if err != nil {
			return nil, err
		}

		if err := c.Establish(); err != nil {
			_ = c.Close()

			return nil, err
		}

		cli := NewClient(c)

		b.clients.Store(addr, cli)

		// Close may have run while the client was dialing.
		if b.closed.Load() {
			b.clients.Delete(addr)
			_ = cli.Close()

			return nil, errors.ErrClientClosed
		}

		return cli, nil
	})
	if err != nil {
		return nil, err
	}

	return cli.(*Client), nil
}

// Close closes every cached client and rejects new builds. It is idempotent.
func (b *Builder) Close() error {
	if b.closed.Swap(true) {
		return nil
	}

	var firstErr error

	b.clients.Range(func(key, value any) bool {
		b.clients.Delete(key)

		if err := value.(*Client).Close(); err != nil && firstErr == nil {
			firstErr = err
		}

		return true
	})

	return firstErr
}
