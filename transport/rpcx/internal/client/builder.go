package client

import (
	"context"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

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

const defaultTimeout = 10 * time.Second

// Builder 客户端连接池构建器，负责创建 rpcx 连接池并管理其生命周期
type Builder struct {
	ctx      context.Context
	cancel   context.CancelFunc
	err      error
	opts     *Options
	dialOpts cli.Option
	builders map[string]resolver.Builder
	sfg      singleflight.Group
	pools    sync.Map
	watcher  registry.Watcher
	closed   atomic.Bool
}

// Options 客户端配置项
type Options struct {
	PoolSize   int
	CAFile     string
	ServerName string
	Dispatch   cluster.Dispatch
	Discovery  registry.Discovery
	FailMode   cli.FailMode
}

// NewBuilder 新建客户端连接池构建器
// 注册直连与服务发现解析器，并按需初始化 TLS 传输配置
// @param opts *Options 客户端配置项
// @return @1 *Builder 构建器实例
func NewBuilder(opts *Options) *Builder {
	b := &Builder{}
	b.opts = opts
	b.builders = make(map[string]resolver.Builder)
	b.dialOpts = cli.DefaultOption
	b.dialOpts.CompressType = proto.Gzip
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.RegisterBuilder(direct.NewBuilder())
	if opts.Discovery != nil {
		b.RegisterBuilder(discovery.NewBuilder())
	}

	if opts.CAFile != "" && opts.ServerName != "" {
		b.dialOpts.TLSConfig, b.err = tls.MakeTCPClientTLSConfig(opts.CAFile, opts.ServerName)
	} else if opts.CAFile != "" || opts.ServerName != "" {
		log.Warn("rpcx client use insecure credentials")
	}

	if b.err == nil {
		if err := b.init(); err != nil {
			b.cancel()
			b.err = err
		}
	}

	return b
}

// RegisterBuilder 注册解析器构建器
// @param builder resolver.Builder 解析器构建器
func (b *Builder) RegisterBuilder(builder resolver.Builder) {
	b.builders[builder.Scheme()] = builder
}

// init 初始化服务发现，加载初始实例并启动实例变更监听
// @return @1 error 错误信息
func (b *Builder) init() error {
	if b.opts.Discovery == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	watcher, err := b.opts.Discovery.Watch(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(b.ctx, defaultTimeout)
	instances, err := b.opts.Discovery.Services(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		_ = watcher.Stop()

		return err
	}

	b.watcher = watcher
	b.updateInstances(instances)

	go b.watch()

	return nil
}

// watch 监听服务实例变更，并同步到各解析器构建器
func (b *Builder) watch() {
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		instances, err := b.watcher.Next()
		if err != nil {
			if errors.Is(err, errors.ErrWatcherStopped) {
				// watcher 已停止，退出循环，避免空转
				return
			}
			// 其他异常，短暂退避后重试，避免忙循环
			time.Sleep(time.Second)
			continue
		}

		b.updateInstances(instances)
	}
}

// updateInstances 更新服务实例状态并分发到各解析器构建器
// @param instances []*registry.ServiceInstance 服务实例列表
func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	for _, builder := range b.builders {
		builder.UpdateStates(instances)
	}
}

// Build 构建客户端
// 相同 target 的连接池会被缓存复用，单飞避免并发重复建池
// @param target string 目标服务地址
// @return @1 *cli.OneClient 客户端实例
// @return @2 error 错误信息
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

		// 防止 Close 与 Build 并发时 Store 进已关闭的连接池
		if b.closed.Load() {
			pool.Close()
			b.pools.Delete(target)
			return nil, errors.ErrClientClosed
		}

		return pool, nil
	})
	if err != nil {
		return nil, err
	}

	return val.(*cli.OneClientPool).Get(), nil
}

// Close 关闭构建器，释放全部连接池与监听资源（幂等）
// @return @1 error 关闭过程中的错误信息
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

	// 通知 watch 协程退出
	if b.cancel != nil {
		b.cancel()
	}

	// 停止服务发现监听，解除 Next() 阻塞
	if b.watcher != nil {
		if err := b.watcher.Stop(); err != nil {
			firstErr = err
		}
	}

	// 关闭解析器构建器，释放监听资源
	for _, builder := range b.builders {
		if err := builder.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
