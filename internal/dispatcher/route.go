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
	eps1       []*serviceEndpoint                 // 所有端点（包含work状态的实例）
	eps2       []*serviceEndpoint                 // 所有端点（包含busy状态的实例）
	eps3       map[string]*serviceEndpoint        // 所有端点（包含work、busy、hang状态的实例）
	route      registry.Route                     // 路由信息
	group      string                             // 路由所属组
	counter    atomic.Uint64                      // 轮询计数器
	wrSeq      atomic.Pointer[[]*serviceEndpoint] // 加权轮询平滑序列（惰性构建）
	dispatcher *Dispatcher                        // 分发器
}

func newRoute(dispatcher *Dispatcher, group string, route registry.Route) *Route {
	r := &Route{
		eps1:       make([]*serviceEndpoint, 0),
		eps2:       make([]*serviceEndpoint, 0),
		eps3:       make(map[string]*serviceEndpoint),
		route:      route,
		group:      group,
		dispatcher: dispatcher,
	}

	return r
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
	if len(insID) > 0 && insID[0] != "" {
		return r.directDispatch(insID[0])
	} else {
		switch r.dispatcher.dispatch {
		case cluster.RoundRobin:
			return r.roundRobinDispatch()
		case cluster.WeightedRoundRobin:
			return r.weightedRoundRobinDispatch()
		default:
			return r.randomDispatch()
		}
	}
}

// 直接分配
func (r *Route) directDispatch(insID string) (*endpoint.Endpoint, error) {
	sep, ok := r.eps3[insID]
	if !ok {
		return nil, errors.ErrNotFoundEndpoint
	}

	return sep.endpoint, nil
}

// 随机分配
func (r *Route) randomDispatch() (*endpoint.Endpoint, error) {
	if eps := r.loadAvailableEndpoints(); len(eps) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	} else {
		return eps[rand.IntN(len(eps))].endpoint, nil
	}
}

// 轮询分配
func (r *Route) roundRobinDispatch() (*endpoint.Endpoint, error) {
	if eps := r.loadAvailableEndpoints(); len(eps) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	} else {
		return eps[(r.counter.Add(1)-1)%uint64(len(eps))].endpoint, nil
	}
}

// 加权轮询分配
// 基于预计算的平滑加权序列与原子游标选择端点，热路径无锁；
// 平滑加权轮询（SWRR）的选择序列具有周期性，预计算与在线计算的分布完全等价
func (r *Route) weightedRoundRobinDispatch() (*endpoint.Endpoint, error) {
	eps := r.loadAvailableEndpoints()
	if len(eps) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	}

	seq := r.loadWRSequence(eps)

	return seq[(r.counter.Add(1)-1)%uint64(len(seq))].endpoint, nil
}

// 加载加权轮询平滑序列
// 序列惰性构建；并发构建产生的内容一致，CAS失败时直接复用已存储值
// @param eps []*serviceEndpoint 可用端点
// @return @1 []*serviceEndpoint 平滑加权序列
func (r *Route) loadWRSequence(eps []*serviceEndpoint) []*serviceEndpoint {
	if seq := r.wrSeq.Load(); seq != nil {
		return *seq
	}

	seq := buildSmoothWRSequence(eps)

	if r.wrSeq.CompareAndSwap(nil, &seq) {
		return seq
	}

	return *r.wrSeq.Load()
}

// 构建平滑加权序列
// 按平滑加权轮询算法离线生成选择序列，权重经GCD归约以控制序列长度；
// 权重未设置（<=0）时兜底为默认权重1，与原在线算法行为一致
// @param eps []*serviceEndpoint 可用端点
// @return @1 []*serviceEndpoint 平滑加权序列
func buildSmoothWRSequence(eps []*serviceEndpoint) []*serviceEndpoint {
	weights := make([]int, len(eps))

	g := 0
	for i, se := range eps {
		weight := se.weight
		if weight <= 0 {
			weight = 1
		}
		weights[i] = weight
		g = gcd(g, weight)
	}

	var total int
	for i := range weights {
		weights[i] /= g
		total += weights[i]
	}

	var (
		curr = make([]int, len(eps))
		seq  = make([]*serviceEndpoint, 0, total)
	)

	for range total {
		selected := 0
		for i := range eps {
			curr[i] += weights[i]
			if curr[i] > curr[selected] {
				selected = i
			}
		}
		curr[selected] -= total
		seq = append(seq, eps[selected])
	}

	return seq
}

// 计算最大公约数
// @param a int 整数a
// @param b int 整数b
// @return @1 int 最大公约数
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}

// 加载可用服务端点
func (r *Route) loadAvailableEndpoints() []*serviceEndpoint {
	switch {
	case len(r.eps1) > 0:
		return r.eps1
	case len(r.eps2) > 0:
		return r.eps2
	default:
		return nil
	}
}

// 添加服务端点
func (r *Route) addServiceEndpoint(se *serviceEndpoint) {
	switch se.state {
	case cluster.Work.String():
		r.eps1 = append(r.eps1, se)
	case cluster.Busy.String():
		r.eps2 = append(r.eps2, se)
	case cluster.Hang.String():
		// ignore
	default:
		return
	}

	r.eps3[se.insID] = se
}
