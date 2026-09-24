package discovery

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

const scheme = "discovery"

// Builder 服务发现模式解析器构建器
// 通过注册中心获取服务实例，并按服务名与实例状态聚合地址
type Builder struct {
	rw        sync.RWMutex
	pairs     map[string][]*cli.KVPair
	resolvers sync.Map
}

// NewBuilder 新建服务发现解析器构建器
// @return @1 *Builder 构建器实例
func NewBuilder() *Builder {
	return &Builder{}
}

// Scheme 获取解析器协议
// @return @1 string 协议名称
func (b *Builder) Scheme() string {
	return scheme
}

// Build 构建服务发现器
// 从缓存状态中查找服务名对应的地址并下发
// @param target *url.URL 目标地址
// @return @1 cli.ServiceDiscovery 服务发现器
// @return @2 error 错误信息
func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
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

// UpdateStates 更新服务实例状态并同步到各服务发现器
// 按实例状态（工作/繁忙/挂起）分组，优先下发高可用性分组，
// 并将实例权重附加到地址值供加权负载均衡使用
// @param instances []*registry.ServiceInstance 服务实例列表
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	var (
		pairs     map[string][]*cli.KVPair
		workPairs = make(map[string][]*cli.KVPair, len(instances))
		busyPairs = make(map[string][]*cli.KVPair, len(instances))
		hangPairs = make(map[string][]*cli.KVPair, len(instances))
	)

	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		switch instance.State {
		case cluster.Work.String():
			pairs = workPairs
		case cluster.Busy.String():
			pairs = busyPairs
		case cluster.Hang.String():
			pairs = hangPairs
		default:
			continue
		}

		for _, service := range instance.Services {
			pairs[service] = append(pairs[service], &cli.KVPair{
				Key:   "tcp@" + ep.Address(),
				Value: fmt.Sprintf("weight=%d", max(1, instance.Weight)),
			})
		}
	}

	// 汇总三个层级中出现过的全部业务服务名
	services := make(map[string]struct{}, len(workPairs)+len(busyPairs)+len(hangPairs))
	for service := range workPairs {
		services[service] = struct{}{}
	}
	for service := range busyPairs {
		services[service] = struct{}{}
	}
	for service := range hangPairs {
		services[service] = struct{}{}
	}

	// 按服务维度独立选择优先级：Work > Busy > Hang
	pairs = make(map[string][]*cli.KVPair, len(services))
	for service := range services {
		switch {
		case workPairs[service] != nil:
			pairs[service] = workPairs[service]
		case busyPairs[service] != nil:
			pairs[service] = busyPairs[service]
		case hangPairs[service] != nil:
			pairs[service] = hangPairs[service]
		}
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

// Close 关闭构建器，释放全部服务发现器
// @return @1 error 错误信息
func (b *Builder) Close() error {
	b.resolvers.Range(func(_, value any) bool {
		value.(*Resolver).Close()
		return true
	})

	return nil
}
