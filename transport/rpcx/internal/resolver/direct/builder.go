package direct

import (
	"context"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

const scheme = "direct"

const defaultTimeout = 10 * time.Second

// Builder 直连模式解析器构建器
// 支持 direct://地址 与 direct://实例ID 两种直连方式
type Builder struct {
	dis       registry.Discovery
	err       error
	ctx       context.Context
	cancel    context.CancelFunc
	watcher   registry.Watcher
	rw        sync.RWMutex
	pairs     map[string][]*cli.KVPair
	resolvers sync.Map
}

// NewBuilder 新建直连解析器构建器
// @param dis registry.Discovery 服务发现组件
// @return @1 *Builder 构建器实例
func NewBuilder(dis registry.Discovery) *Builder {
	b := &Builder{}
	b.dis = dis
	b.ctx, b.cancel = context.WithCancel(context.Background())

	if err := b.init(); err != nil {
		b.err = err
	}

	return b
}

// Scheme 获取解析器协议
// @return @1 string 协议名称
func (b *Builder) Scheme() string {
	return scheme
}

// Build 构建服务发现器
// 地址可直接解析为 host:port 时返回点对点发现器，否则按实例ID查找缓存地址
// @param target *url.URL 目标地址
// @return @1 cli.ServiceDiscovery 服务发现器
// @return @2 error 错误信息
func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
	if b.err != nil {
		return nil, b.err
	}

	if _, _, err := net.SplitHostPort(target.Host); err == nil {
		return cli.NewPeer2PeerDiscovery("tcp@"+target.Host, "")
	} else {
		b.rw.RLock()
		pairs, ok := b.pairs[target.Host]
		b.rw.RUnlock()
		if !ok {
			return nil, errors.ErrNotFoundServiceAddress
		}

		r := newResolver(target.Host, b)
		r.updateState(pairs)

		b.resolvers.Store(target.Host, r)

		return r, nil
	}
}

// init 初始化服务发现，加载初始实例并启动实例变更监听
// @return @1 error 错误信息
func (b *Builder) init() error {
	if b.dis == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(b.ctx, defaultTimeout)
	watcher, err := b.dis.Watch(ctx, cluster.Mesh.String())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(b.ctx, defaultTimeout)
	instances, err := b.dis.Services(ctx, cluster.Mesh.String())
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

// watch 监听服务实例变更，并同步到各服务发现器
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

// updateInstances 更新服务实例状态并同步到各服务发现器
// 将实例端点按实例ID聚合为地址对，实例下线时下发空状态
// @param instances []*registry.ServiceInstance 服务实例列表
func (b *Builder) updateInstances(instances []*registry.ServiceInstance) {
	pairs := make(map[string][]*cli.KVPair, len(instances))
	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		pairs[instance.ID] = append(pairs[instance.ID], &cli.KVPair{Key: "tcp@" + ep.Address()})
	}

	b.rw.Lock()
	b.pairs = pairs
	b.rw.Unlock()

	b.resolvers.Range(func(_, value any) bool {
		r := value.(*Resolver)
		r.updateState(pairs[r.name])

		return true
	})
}

// removeResolver 移除服务发现器
// @param r *Resolver 服务发现器
func (b *Builder) removeResolver(r *Resolver) {
	b.resolvers.Delete(r.name)
}

// Close 关闭构建器，释放 watch 协程与监听资源
// @return @1 error 错误信息
func (b *Builder) Close() (err error) {
	// 通知 watch 协程退出
	if b.cancel != nil {
		b.cancel()
	}

	// 停止服务发现监听，解除 Next() 阻塞
	if b.watcher != nil {
		err = b.watcher.Stop()
	}

	// 关闭全部解析器
	b.resolvers.Range(func(_, value any) bool {
		value.(*Resolver).Close()
		return true
	})

	return
}
