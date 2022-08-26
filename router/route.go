package router

import (
	"github.com/dobyte/due/internal/endpoint"
	"sync"
)

const (
	balanceStrategyRandom           = "random" // 随机
	balanceStrategyRoundRobin       = "rr"     // 轮询
	balanceStrategyWeightRoundRobin = "wrr"    // 加权轮询
)

type Route struct {
	id              int32    // 路由ID
	stateful        bool     // 是否有状态
	endpoints       sync.Map // 服务端口
	balanceStrategy string   // 负载均衡策略
}

type Endpoint = endpoint.Endpoint

// Stateful 是否有状态
func (r *Route) Stateful() bool {
	return r.stateful
}

// FindEndpoint 查询路由服务端口
func (r *Route) FindEndpoint(insID string) (*Endpoint, error) {
	if insID == "" {
		switch r.balanceStrategy {
		case balanceStrategyRandom:
			return r.randomDispatch()
		case balanceStrategyRoundRobin:
			return r.roundRobinDispatch()
		default:
			return r.randomDispatch()
		}
	}

	val, ok := r.endpoints.Load(insID)
	if !ok {
		return nil, ErrNotFoundEndpoint
	}

	return val.(*Endpoint), nil
}

// 随机分配
func (r *Route) randomDispatch() (ep *Endpoint, err error) {
	r.endpoints.Range(func(_, val interface{}) bool {
		ep = val.(*Endpoint)
		return false
	})

	if ep == nil {
		err = ErrNotFoundEndpoint
	}

	return
}

// 轮询分配
func (r *Route) roundRobinDispatch() (ep *Endpoint, err error) {
	return r.randomDispatch()
}
