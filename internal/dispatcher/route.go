package dispatcher

type Route struct {
	abstract
	id       int32  // 路由ID
	group    string // 路由所属组
	stateful bool   // 是否有状态
}

func newRoute(dispatcher *Dispatcher, id int32, group string, stateful bool) *Route {
	return &Route{
		id:       id,
		group:    group,
		stateful: stateful,
		abstract: abstract{
			dispatcher:  dispatcher,
			endpointMap: make(map[string]*serviceEndpoint),
			endpointArr: make([]*serviceEndpoint, 0),
		},
	}
}

// ID 获取路由ID
func (r *Route) ID() int32 {
	return r.id
}

// Group 路由所属组
func (r *Route) Group() string {
	return r.group
}

// Stateful 获取路由状态
func (r *Route) Stateful() bool {
	return r.stateful
}
