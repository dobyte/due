package direct

import (
	"net"
	"sync"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

const scheme = "direct"

// Builder 直连模式解析器构建器
// 支持 direct://地址 与 direct://实例ID 两种直连方式
type Builder struct {
	rw        sync.RWMutex
	states    map[string]*resolver.State
	resolvers sync.Map
}

var _ resolver.Builder = &Builder{}

// NewBuilder 新建直连解析器构建器
// @return @1 *Builder 构建器实例
func NewBuilder() *Builder {
	return &Builder{states: make(map[string]*resolver.State)}
}

// Build 构建解析器
// 地址可直接解析为 host:port，实例ID则从缓存状态中查找对应地址
// @param target resolver.Target 目标
// @param cc resolver.ClientConn 客户端连接
// @param opts resolver.BuildOptions 构建选项
// @return @1 resolver.Resolver 解析器实例
// @return @2 error 错误信息
func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &Resolver{builder: b, target: target, cc: cc}

	if _, _, err := net.SplitHostPort(target.URL.Host); err == nil {
		r.updateState(resolver.State{Addresses: []resolver.Address{{Addr: target.URL.Host}}})
	} else {
		b.rw.RLock()
		state := b.states[target.URL.Host]
		b.rw.RUnlock()

		if state != nil {
			r.updateState(*state)
		} else {
			r.updateState(resolver.State{})
		}

		b.resolvers.Store(target.URL.Host, r)
	}

	return r, nil
}

// Scheme 获取解析器协议
// @return @1 string 协议名称
func (b *Builder) Scheme() string {
	return scheme
}

// UpdateStates 更新服务实例状态并同步到各解析器
// 将实例端点按实例ID聚合为地址状态，实例下线时下发空状态
// @param instances []*registry.ServiceInstance 服务实例列表
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
	states := make(map[string]*resolver.State, len(instances))
	for _, instance := range instances {
		ep, err := endpoint.ParseEndpoint(instance.Endpoint)
		if err != nil {
			log.Errorf("parse discovery endpoint failed: %v", err)
			continue
		}

		if state, ok := states[instance.ID]; ok {
			state.Addresses = append(state.Addresses, resolver.Address{Addr: ep.Address()})
		} else {
			states[instance.ID] = &resolver.State{Addresses: []resolver.Address{{Addr: ep.Address()}}}
		}
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
