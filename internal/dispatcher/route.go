package dispatcher

import (
	"math/rand/v2"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

type Route struct {
	abstract
	route      registry.Route // 路由信息
	group      string         // 路由所属组
	counter    atomic.Uint64  // 轮询计数器
	dispatcher *Dispatcher    // 分发器
}

func newRoute(dispatcher *Dispatcher, group string, route registry.Route) *Route {
	return &Route{
		route:      route,
		group:      group,
		dispatcher: dispatcher,
		abstract:   newAbstract(),
	}
}

// ID 获取路由ID
func (r *Route) ID() int32 {
	return r.route.ID
}

// Group 路由所属组
func (r *Route) Group() string {
	return r.group
}

// Internal 是否内部路由
func (r *Route) Internal() bool {
	return r.route.Internal
}

// Stateful 是否有状态路由
func (r *Route) Stateful() bool {
	return r.route.Stateful
}

// Authorized 是否授权路由
func (r *Route) Authorized() bool {
	return r.route.Authorized
}

// FindEndpoint 查询路由服务端点
func (r *Route) FindEndpoint(insID ...string) (*endpoint.Endpoint, error) {
	if len(insID) == 0 || insID[0] == "" {
		switch r.dispatcher.dispatch {
		case cluster.RoundRobin:
			return r.roundRobinDispatch()
		case cluster.WeightRoundRobin:
			return r.weightRoundRobinDispatch()
		default:
			return r.randomDispatch()
		}
	}

	return r.directDispatch(insID[0])
}

// 直接分配
func (r *Route) directDispatch(insID string) (*endpoint.Endpoint, error) {
	sep, ok := r.endpoints5[insID]
	if !ok {
		return nil, errors.ErrNotFoundEndpoint
	}

	return sep.endpoint, nil
}

// 随机分配
func (r *Route) randomDispatch() (*endpoint.Endpoint, error) {
	if n := len(r.endpoints1); n > 0 {
		return r.endpoints1[rand.IntN(n)].endpoint, nil
	}

	if n := len(r.endpoints2); n > 0 {
		return r.endpoints2[rand.IntN(n)].endpoint, nil
	}

	return nil, errors.ErrNotFoundEndpoint
}

// 轮询分配
func (r *Route) roundRobinDispatch() (*endpoint.Endpoint, error) {
	if n := len(r.endpoints1); n > 0 {
		index := int(r.counter.Add(1) % uint64(n))

		return r.endpoints1[index].endpoint, nil
	}

	if n := len(r.endpoints2); n > 0 {
		index := int(r.counter.Add(1) % uint64(n))

		return r.endpoints2[index].endpoint, nil
	}

	return nil, errors.ErrNotFoundEndpoint
}

// 加权轮询分配
func (r *Route) weightRoundRobinDispatch() (*endpoint.Endpoint, error) {
	var (
		selected    *serviceEndpoint
		totalWeight int
	)

	if len(r.endpoints1) > 0 {
		for i := range r.endpoints1 {
			se := r.endpoints1[i]
			se.currWeight += se.weight

			totalWeight += se.weight

			if selected == nil || se.currWeight > selected.currWeight {
				selected = se
			}
		}

		selected.currWeight -= totalWeight

		return selected.endpoint, nil
	}

	if len(r.endpoints2) > 0 {
		for i := range r.endpoints2 {
			se := r.endpoints2[i]
			se.currWeight += se.weight

			totalWeight += se.weight

			if selected == nil || se.currWeight > selected.currWeight {
				selected = se
			}
		}

		selected.currWeight -= totalWeight

		return selected.endpoint, nil
	}

	return nil, errors.ErrNotFoundEndpoint
}
