package gate

import (
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/client"
	"github.com/dobyte/due/v2/utils/xtime"
	"golang.org/x/sync/singleflight"
)

const defaultFaultTimeout = 3 * time.Second // 默认故障超时时间

type Options struct {
	InsID   string       // 实例ID
	InsKind cluster.Kind // 实例类型
}

type Builder struct {
	sfg     singleflight.Group
	opts    *Options
	faults  sync.Map
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

		if t, ok := b.faults.Load(addr); ok && xtime.Now().Sub(t.(xtime.Time)) <= defaultFaultTimeout {
			return nil, errors.ErrServerClosed
		}

		c := client.NewClient(&client.Options{
			Addr:    addr,
			InsID:   b.opts.InsID,
			InsKind: b.opts.InsKind,
			CloseHandler: func() {
				b.faults.Store(addr, xtime.Now())
				b.clients.Delete(addr)
			},
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
