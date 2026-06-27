package dispatcher

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
)

type Event struct {
	eps   map[string]*serviceEndpoint // 所有端点（包含work、busy、hang、shut状态的实例）
	event int                         // 事件ID
}

func newEvent(event int) *Event {
	return &Event{
		eps:   make(map[string]*serviceEndpoint),
		event: event,
	}
}

// Event 获取事件
func (e *Event) Event() int {
	return e.event
}

// VisitEndpoints 迭代服务端口
func (e *Event) VisitEndpoints(fn func(insID string, ep *endpoint.Endpoint) bool) {
	for insID, se := range e.eps {
		if !fn(insID, se.endpoint) {
			return
		}
	}
}

// 添加服务端点
func (e *Event) addServiceEndpoint(se *serviceEndpoint) {
	if se.state != cluster.Shut.String() {
		e.eps[se.insID] = se
	}
}
