package client

import (
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver/direct"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver/discovery"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/def"
	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
	proto "github.com/smallnest/rpcx/protocol"
	"golang.org/x/sync/singleflight"
)

const defaultPoolSize = 10

type Builder struct {
	err      error
	opts     *Options
	dialOpts cli.Option
	builders map[string]resolver.Builder
	sfg      singleflight.Group
	pools    sync.Map
	closed   atomic.Bool
}

type Options struct {
	PoolSize   int
	CAFile     string
	ServerName string
	Dispatch   cluster.Dispatch
	Discovery  registry.Discovery
	FailMode   cli.FailMode
}

func NewBuilder(opts *Options) *Builder {
	b := &Builder{}
	b.opts = opts
	b.builders = make(map[string]resolver.Builder)
	b.dialOpts = cli.DefaultOption
	b.dialOpts.CompressType = proto.Gzip
	b.RegisterBuilder(direct.NewBuilder(opts.Discovery))
	if opts.Discovery != nil {
		b.RegisterBuilder(discovery.NewBuilder(opts.Discovery))
	}

	if opts.CAFile != "" && opts.ServerName != "" {
		b.dialOpts.TLSConfig, b.err = tls.MakeTCPClientTLSConfig(opts.CAFile, opts.ServerName)
	} else if opts.CAFile != "" || opts.ServerName != "" {
		log.Warn("grpc client use insecure credentials")
	}

	return b
}

// RegisterBuilder 注册构建器
func (b *Builder) RegisterBuilder(builder resolver.Builder) {
	b.builders[builder.Scheme()] = builder
}

// Build 建立Discovery
func (b *Builder) Build(target string) (*cli.OneClient, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.closed.Load() {
		return nil, errors.ErrClientClosed
	}

	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	val, ok := b.pools.Load(target)
	if ok {
		return val.(*cli.OneClientPool).Get(), nil
	}

	val, err, _ = b.sfg.Do(target, func() (any, error) {
		if b.closed.Load() {
			return nil, errors.ErrClientClosed
		}

		builder, ok := b.builders[u.Scheme]
		if !ok {
			return nil, errors.ErrMissingResolver
		}

		dis, err := builder.Build(u)
		if err != nil {
			return nil, err
		}

		size := b.opts.PoolSize
		if size <= 0 {
			size = defaultPoolSize
		}

		var selectMode cli.SelectMode
		switch b.opts.Dispatch {
		case def.Random:
			selectMode = cli.RandomSelect
		case def.WeightedRoundRobin:
			selectMode = cli.WeightedRoundRobin
		case def.ConsistentHash:
			selectMode = cli.ConsistentHash
		default:
			selectMode = cli.RoundRobin
		}

		pool := cli.NewOneClientPool(size, b.opts.FailMode, selectMode, dis, b.dialOpts)

		b.pools.Store(target, pool)

		return pool, nil
	})
	if err != nil {
		return nil, err
	}

	return val.(*cli.OneClientPool).Get(), nil
}

// Close 关闭构建器，释放全部连接池与监听资源（幂等）
func (b *Builder) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	var firstErr error

	// 关闭全部连接池
	b.pools.Range(func(_, value any) bool {
		value.(*cli.OneClientPool).Close()
		return true
	})
	b.pools.Clear()

	// 关闭解析器构建器，释放 watch 协程与监听资源
	for _, builder := range b.builders {
		if err := builder.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
