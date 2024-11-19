package dispatcher

type Route struct {
	abstract
	id       int32  // 路由ID
	group    string // 路由所属组
	stateful bool   // 是否有状态
	internal bool   // 是否内部路由
}

func newRoute(dispatcher *Dispatcher, id int32, group string, stateful, internal bool) *Route {
	return &Route{
		id:       id,
		group:    group,
		stateful: stateful,
		internal: internal,
		abstract: abstract{
			dispatcher: dispatcher,
			endpoints1: make([]*serviceEndpoint, 0),
			endpoints2: make(map[string]*serviceEndpoint),
			endpoints3: make([]*serviceEndpoint, 0),
			endpoints4: make(map[string]*serviceEndpoint),
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

// Internal 是否内部路由
func (r *Route) Internal() bool {
	return r.internal
}
