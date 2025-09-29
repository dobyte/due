package gate

import (
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/internal/client"
	"golang.org/x/sync/singleflight"
)

type Options struct {
	InsID   string       // 实例ID
	InsKind cluster.Kind // 实例类型
}

type Builder struct {
	sfg     singleflight.Group
	opts    *Options
	clients sync.Map
}

func NewBuilder(opts *Options) *Builder {
	return &Builder{
		opts: opts,
	}
}

// Build 构建客户端
func (b *Builder) Build(addr string) (*Client, error) {
	if cli, ok := b.clients.Load(addr); ok {
		return cli.(*Client), nil
	}

	cli, err, _ := b.sfg.Do(addr, func() (any, error) {
		if cli, ok := b.clients.Load(addr); ok {
			return cli.(*Client), nil
		}

		c := client.NewClient(&client.Options{
			Addr:         addr,
			InsID:        b.opts.InsID,
			InsKind:      b.opts.InsKind,
			CloseHandler: func() { b.clients.Delete(addr) },
		})

		if err := c.Establish(); err != nil {
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
