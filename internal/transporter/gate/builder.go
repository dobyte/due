package gate

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"golang.org/x/sync/singleflight"
)

type ClientOptions = drpc.ClientOptions

type Builder struct {
	locals  map[string]*localClient
	mu      sync.RWMutex
	closed  bool
	sfg     singleflight.Group
	opts    *ClientOptions
	clients sync.Map
}

func NewBuilder(opts *ClientOptions) *Builder {
	return &Builder{
		opts:   opts,
		locals: make(map[string]*localClient),
	}
}

// Build returns a client for addr, reusing a cached client when one already exists.
func (b *Builder) Build(addr string) (*Client, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errors.ErrClientClosed
	}
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
		if err := c.Establish(ctx); err != nil {
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

// Close prevents new connections and closes every cached remote client.
func (b *Builder) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	clear(b.locals)
	b.clients.Range(func(key, value any) bool {
		if c, ok := value.(*Client); ok {
			_ = c.cli.Close()
		}
		b.clients.Delete(key)
		return true
	})
}
