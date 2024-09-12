package node

type actorOptions struct {
	id   string // Actor编号
	args []any  // 传递到Processor中的参数
}

type ActorOption func(o *actorOptions)

func defaultActorOptions() *actorOptions {
	return &actorOptions{}
}

// WithActorID 设置Actor编号
func WithActorID(id string) ActorOption {
	return func(o *actorOptions) { o.id = id }
}

// WithActorArgs 设置传递到Processor中的参数
func WithActorArgs(args ...any) ActorOption {
	return func(o *actorOptions) { o.args = append(o.args, args...) }
}
