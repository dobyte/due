package direct

import (
	"net"
	"net/url"
	"sync"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
)

const scheme = "direct"

// Builder 直连模式解析器构建器
// 支持 direct://地址 与 direct://实例ID 两种直连方式
type Builder struct {
	rw        sync.RWMutex
	pairs     map[string][]*cli.KVPair
	resolvers sync.Map
}

// NewBuilder 新建直连解析器构建器
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
// 地址可直接解析为 host:port 时返回点对点发现器，否则按实例ID查找缓存地址
// @param target *url.URL 目标地址
// @return @1 cli.ServiceDiscovery 服务发现器
// @return @2 error 错误信息
func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
	if _, _, err := net.SplitHostPort(target.Host); err == nil {
		return cli.NewPeer2PeerDiscovery("tcp@"+target.Host, "")
	}

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
// 将实例端点按实例ID聚合为地址对，实例下线时下发空状态
// @param instances []*registry.ServiceInstance 服务实例列表
func (b *Builder) UpdateStates(instances []*registry.ServiceInstance) {
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

// Close 关闭构建器，释放全部服务发现器
// @return @1 error 错误信息
func (b *Builder) Close() error {
	b.resolvers.Range(func(_, value any) bool {
		value.(*Resolver).Close()
		return true
	})

	return nil
}
