package node

type actorOptions struct {
	id   string
	args []any
}

type ActorOption func(o *actorOptions)

func defaultActorOptions() *actorOptions {
	return &actorOptions{}
}

func WithActorID(id string) ActorOption {
	return func(o *actorOptions) { o.id = id }
}

func WithActorArgs(args ...any) ActorOption {
	return func(o *actorOptions) { o.args = append(o.args, args...) }
}
