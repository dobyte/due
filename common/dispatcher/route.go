package dispatcher

type Route struct {
	abstract
	id       int32 // 路由ID
	stateful bool  // 是否有状态
}

func newRoute(dispatcher *Dispatcher, id int32, stateful bool) *Route {
	return &Route{
		id:       id,
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

// Stateful 获取路由状态
func (r *Route) Stateful() bool {
	return r.stateful
}
