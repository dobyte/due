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
	endpoints1 []*serviceEndpoint          // 所有端点（包含work、busy、hang、shut状态的实例）
	endpoints2 map[string]*serviceEndpoint // 所有端点（包含work、busy、hang、shut状态的实例）
	endpoints3 []*serviceEndpoint          // 所有端点（包含work、busy状态的实例）
	endpoints4 map[string]*serviceEndpoint // 所有端点（包含work、busy状态的实例）
}

func newAbstract() abstract {
	return abstract{
		endpoints1: make([]*serviceEndpoint, 0),
		endpoints2: make(map[string]*serviceEndpoint),
		endpoints3: make([]*serviceEndpoint, 0),
		endpoints4: make(map[string]*serviceEndpoint),
	}
}

// 添加服务端点
func (a *abstract) addServiceEndpoint(se *serviceEndpoint) {
	a.endpoints1 = append(a.endpoints1, se)
	a.endpoints2[se.insID] = se

	if se.state == cluster.Work.String() || se.state == cluster.Busy.String() {
		a.endpoints3 = append(a.endpoints3, se)
		a.endpoints4[se.insID] = se
	}
}
