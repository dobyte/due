package node

type baseActor struct {
}

type Engine struct {
	base
	router  *Router
	trigger *Trigger
}
