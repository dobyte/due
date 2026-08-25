package discovery

import (
	"sync"

	"github.com/dobyte/due/transport/grpc/v2/internal/balancer/wrr"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
)

const scheme = "discovery"

// Builder 服务发现模式解析器构建器
// 通过注册中心获取服务实例，并按服务名与实例状态聚合地址
type Builder struct {
	rw        sync.RWMutex
	states    map[string]*resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

// NewBuilder 新建服务发现解析器构建器
// @return @1 *Builder 构建器实例
func NewBuilder() *Builder {
	return &Builder{states: make(map[string]*resolver.State)}
}

// Build 构建解析器
// 从缓存状态中查找服务名对应的地址并下发
// @param target resolver.Target 目标
// @param cc resolver.ClientConn 客户端连接
// @param opts resolver.BuildOptions 构建选项
// @return @1 resolver.Resolver 解析器实例
// @return @2 error 错误信息
func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	b.rw.RLock()
	state := b.states[target.URL.Host]
	b.rw.RUnlock()

	r := &Resolver{builder: b, target: target, cc: cc}

	if state != nil {
		r.updateState(*state)
	} else {
		r.updateState(resolver.State{})
	}

	b.resolvers.Store(target.URL.Host, r)

	return r, nil
}

// Scheme 获取解析器协议
// @return @1 string 协议名称
func (b *Builder) Scheme() string {
	return scheme
}

// UpdateStates 更新服务实例状态并同步到各解析器
// 按实例状态（工作/繁忙/挂起）分组，优先下发高可用性分组，
// 并将实例权重附加到地址属性供加权负载均衡使用
// @param instances []*registry.ServiceInstance 服务实例列表
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	var (
		states     map[string]*resolver.State
		workStates = make(map[string]*resolver.State, len(instances))
		busyStates = make(map[string]*resolver.State, len(instances))
		hangStates = make(map[string]*resolver.State, len(instances))
	)

	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		switch instance.State {
		case cluster.Work.String():
			states = workStates
		case cluster.Busy.String():
			states = busyStates
		case cluster.Hang.String():
			states = hangStates
		default:
			continue
		}

		for _, service := range instance.Services {
			addr := resolver.Address{
				Addr:       ep.Address(),
				ServerName: service,
				Attributes: attributes.New(wrr.WeightAttrKey, uint32(max(1, instance.Weight))),
			}

			if state, ok := states[service]; ok {
				state.Addresses = append(state.Addresses, addr)
			} else {
				states[service] = &resolver.State{Addresses: []resolver.Address{addr}}
			}
		}
	}

	switch {
	case len(workStates) > 0:
		states = workStates
	case len(busyStates) > 0:
		states = busyStates
	case len(hangStates) > 0:
		states = hangStates
	default:
		states = make(map[string]*resolver.State)
	}

	b.rw.Lock()
	b.states = states
	b.rw.Unlock()

	b.resolvers.Range(func(key, value any) bool {
		r := value.(*Resolver)
		if state, ok := states[r.target.URL.Host]; ok {
			r.updateState(*state)
		} else {
			r.updateState(resolver.State{})
		}
		return true
	})
}
