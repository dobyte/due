package node

type actorOptions struct {
	id       string // Actor编号
	kind     string // Actor类型
	args     []any  // 传递到Processor中的参数
	wait     bool   // 是否需要等待
	dispatch bool   // 是否接受调度器调度
}

type ActorOption func(o *actorOptions)

func defaultActorOptions() *actorOptions {
	return &actorOptions{wait: true, dispatch: true}
}

// WithActorID 设置Actor编号
func WithActorID(id string) ActorOption {
	return func(o *actorOptions) { o.id = id }
}

// WithActorKind 设置Actor类型
func WithActorKind(kind string) ActorOption {
	return func(o *actorOptions) { o.kind = kind }
}

// WithActorArgs 设置传递到Processor中的参数
func WithActorArgs(args ...any) ActorOption {
	return func(o *actorOptions) { o.args = append(o.args, args...) }
}

// WithActorNonWait 设置Actor无需等待属性（Node组件关关闭时无需等待此Actor结束）
func WithActorNonWait() ActorOption {
	return func(o *actorOptions) { o.wait = false }
}

// WithActorNonDispatch 设置Actor不可调度
func WithActorNonDispatch() ActorOption {
	return func(o *actorOptions) { o.dispatch = false }
}
