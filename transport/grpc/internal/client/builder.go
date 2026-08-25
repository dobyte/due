package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/dobyte/due/transport/grpc/v2/internal/balancer/ch"
	_ "github.com/dobyte/due/transport/grpc/v2/internal/balancer/random"
	_ "github.com/dobyte/due/transport/grpc/v2/internal/balancer/wrr"
	iresolver "github.com/dobyte/due/transport/grpc/v2/internal/resolver"
	"github.com/dobyte/due/transport/grpc/v2/internal/resolver/direct"
	"github.com/dobyte/due/transport/grpc/v2/internal/resolver/discovery"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/def"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

const defaultTimeout = 10 * time.Second

// Options 客户端配置项
type Options struct {
	CAFile     string
	ServerName string
	Dispatch   def.Dispatch
	Discovery  registry.Discovery
	DialOpts   []grpc.DialOption
}

// Builder 客户端连接构建器，负责创建 gRPC 连接并管理其生命周期
type Builder struct {
	ctx         context.Context
	cancel      context.CancelFunc
	err         error
	opts        *Options
	dialOpts    []grpc.DialOption
	resolvers   []resolver.Builder
	sfg         singleflight.Group
	connections sync.Map
	watcher     registry.Watcher
	closed      atomic.Bool
}

// NewBuilder 新建客户端连接构建器
// 根据配置初始化传输凭证、解析器与负载均衡策略
// @param opts *Options 客户端配置项
// @return @1 *Builder 构建器实例
func NewBuilder(opts *Options) *Builder {
	var (
		err  error
		cred credentials.TransportCredentials
	)

	if opts.CAFile != "" && opts.ServerName != "" {
		if cred, err = credentials.NewClientTLSFromFile(opts.CAFile, opts.ServerName); err != nil {
			return &Builder{err: err}
		}
	} else {
		if opts.CAFile != "" || opts.ServerName != "" {
			log.Warn("grpc client use insecure credentials")
		}

		cred = insecure.NewCredentials()
	}

	resolvers := make([]resolver.Builder, 0, 2)
	resolvers = append(resolvers, direct.NewBuilder())

	if opts.Discovery != nil {
		resolvers = append(resolvers, discovery.NewBuilder())
	}

	b := &Builder{}
	b.opts = opts
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.resolvers = resolvers
	b.dialOpts = make([]grpc.DialOption, 0, len(opts.DialOpts)+2)
	b.dialOpts = append(b.dialOpts, opts.DialOpts...)
	b.dialOpts = append(b.dialOpts, grpc.WithTransportCredentials(cred))
	b.dialOpts = append(b.dialOpts, grpc.WithResolvers(resolvers...))

	switch opts.Dispatch {
	case def.Random:
		b.dialOpts = append(b.dialOpts, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"random":{}}]}`))
	case def.WeightedRoundRobin:
		b.dialOpts = append(b.dialOpts, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"wrr":{}}]}`))
	case def.ConsistentHash:
		b.dialOpts = append(b.dialOpts, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"ch":{}}]}`))
	default:
		b.dialOpts = append(b.dialOpts, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`))
	}

	if err := b.init(); err != nil {
		return &Builder{err: err}
	}

	return b
}

// init 初始化服务发现，加载初始实例并启动实例变更监听
// @return @1 error 错误信息
func (b *Builder) init() error {
	if b.opts.Discovery == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	instances, err := b.opts.Discovery.Services(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(b.ctx, defaultTimeout)
	watcher, err := b.opts.Discovery.Watch(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	b.watcher = watcher
	b.updateInstances(instances)

	go b.watch()

	return nil
}

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
				return
			}
			time.Sleep(time.Second)
			continue
		}

		b.updateInstances(instances)
	}
}

// updateInstances 更新服务实例
func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	for _, r := range b.resolvers {
		r.(iresolver.Builder).UpdateStates(instances)
	}
}

// Build 构建连接
// 相同 target 的连接会被缓存复用，单飞避免并发重复建连
// @param target string 目标服务地址
// @return @1 *grpc.ClientConn 客户端连接
// @return @2 error 错误信息
func (b *Builder) Build(target string) (*grpc.ClientConn, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.closed.Load() {
		return nil, errors.ErrClientClosed
	}

	if c, ok := b.connections.Load(target); ok {
		return c.(*grpc.ClientConn), nil
	}

	c, err, _ := b.sfg.Do(target, func() (any, error) {
		if b.closed.Load() {
			return nil, errors.ErrClientClosed
		}

		cc, err := grpc.NewClient(target, b.dialOpts...)
		if err != nil {
			return nil, err
		}

		b.connections.Store(target, cc)

		// 防止 Close 与 Build 并发时 Store 进已关闭的连接
		if b.closed.Load() {
			_ = cc.Close()
			b.connections.Delete(target)
			return nil, errors.ErrClientClosed
		}

		return cc, nil
	})
	if err != nil {
		return nil, err
	}

	return c.(*grpc.ClientConn), nil
}

// Close 关闭构建器，释放全部连接与资源（幂等）
func (b *Builder) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	// 通知 watch 协程退出
	if b.cancel != nil {
		b.cancel()
	}

	// 停止服务发现监听，触发 Next() 返回 ErrWatcherStopped
	if b.watcher != nil {
		_ = b.watcher.Stop()
	}

	// 关闭全部连接并清空缓存
	var firstErr error
	b.connections.Range(func(key, value any) bool {
		if err := value.(*grpc.ClientConn).Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		b.connections.Delete(key)
		return true
	})

	return firstErr
}
