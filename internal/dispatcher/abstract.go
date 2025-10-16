package dispatcher

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
)

type serviceEndpoint struct {
	insID      string
	state      string
	endpoint   *endpoint.Endpoint
	weight     int
	currWeight int
}

type abstract struct {
	endpoints1 []*serviceEndpoint          // 所有端点（包含work状态的实例）
	endpoints2 []*serviceEndpoint          // 所有端点（包含busy状态的实例）
	endpoints3 []*serviceEndpoint          // 所有端点（包含hang状态的实例）
	endpoints4 []*serviceEndpoint          // 所有端点（包含shut状态的实例）
	endpoints5 map[string]*serviceEndpoint // 所有端点（包含work、busy、hang、shut状态的实例）
}

func newAbstract() abstract {
	return abstract{
		endpoints1: make([]*serviceEndpoint, 0),
		endpoints2: make([]*serviceEndpoint, 0),
		endpoints3: make([]*serviceEndpoint, 0),
		endpoints4: make([]*serviceEndpoint, 0),
		endpoints5: make(map[string]*serviceEndpoint),
	}
}

// 添加服务端点
func (a *abstract) addServiceEndpoint(se *serviceEndpoint) {
	switch se.state {
	case cluster.Work.String():
		a.endpoints1 = append(a.endpoints1, se)
	case cluster.Busy.String():
		a.endpoints2 = append(a.endpoints2, se)
	case cluster.Hang.String():
		a.endpoints3 = append(a.endpoints3, se)
	case cluster.Shut.String():
		a.endpoints4 = append(a.endpoints4, se)
	}

	a.endpoints5[se.insID] = se
}
