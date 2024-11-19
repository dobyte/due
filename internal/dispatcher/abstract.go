package dispatcher

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"sync/atomic"
)

type serviceEndpoint struct {
	insID    string
	state    string
	endpoint *endpoint.Endpoint
}

type abstract struct {
	counter    atomic.Uint64
	dispatcher *Dispatcher
	endpoints1 []*serviceEndpoint          // 所有端口（包含work、busy、hang、shut状态的实例）
	endpoints2 map[string]*serviceEndpoint // 所有端口（包含work、busy、hang、shut状态的实例）
	endpoints3 []*serviceEndpoint          // 所有端口（包含work、busy状态的实例）
	endpoints4 map[string]*serviceEndpoint // 所有端口（包含work、busy状态的实例）
}

// FindEndpoint 查询路由服务端点
func (a *abstract) FindEndpoint(insID ...string) (*endpoint.Endpoint, error) {
	if len(insID) == 0 || insID[0] == "" {
		switch a.dispatcher.strategy {
		case RoundRobin:
			return a.roundRobinDispatch()
		case WeightRoundRobin:
			return a.randomDispatch()
		default:
			return a.randomDispatch()
		}
	}

	return a.directDispatch(insID[0])
}

// IterateEndpoint 迭代服务端口
func (a *abstract) IterateEndpoint(fn func(insID string, ep *endpoint.Endpoint) bool) {
	for _, se := range a.endpoints1 {
		if fn(se.insID, se.endpoint) == false {
			break
		}
	}
}

// 添加服务端点
func (a *abstract) addEndpoint(insID string, state string, endpoint *endpoint.Endpoint) {
	if se, ok := a.endpoints2[insID]; ok {
		se.state = state
		se.endpoint = endpoint
	} else {
		se = &serviceEndpoint{insID: insID, state: state, endpoint: endpoint}
		a.endpoints1 = append(a.endpoints1, se)
		a.endpoints2[insID] = se
	}

	switch state {
	case cluster.Work.String(), cluster.Busy.String():
		if se, ok := a.endpoints4[insID]; ok {
			se.state = state
			se.endpoint = endpoint
		} else {
			se = &serviceEndpoint{insID: insID, state: state, endpoint: endpoint}
			a.endpoints3 = append(a.endpoints3, se)
			a.endpoints4[insID] = se
		}
	case cluster.Hang.String():
		if _, ok := a.endpoints4[insID]; ok {
			delete(a.endpoints4, insID)

			for i, se := range a.endpoints3 {
				if se.insID == insID {
					a.endpoints3 = append(a.endpoints3[:i], a.endpoints3[i+1:]...)
					break
				}
			}
		}
	}
}

// 直接分配
func (a *abstract) directDispatch(insID string) (*endpoint.Endpoint, error) {
	sep, ok := a.endpoints2[insID]
	if !ok {
		return nil, errors.ErrNotFoundEndpoint
	}

	return sep.endpoint, nil
}

// 随机分配
func (a *abstract) randomDispatch() (*endpoint.Endpoint, error) {
	for _, sep := range a.endpoints4 {
		return sep.endpoint, nil
	}

	return nil, errors.ErrNotFoundEndpoint
}

// 轮询分配
func (a *abstract) roundRobinDispatch() (*endpoint.Endpoint, error) {
	if len(a.endpoints3) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	}

	index := int(a.counter.Add(1) % uint64(len(a.endpoints3)))

	return a.endpoints3[index].endpoint, nil
}
