package node

type actorOptions struct {
	id   string
	kind string
}

type ActorOption func(o *actorOptions)

func WithActorID(id string) ActorOption {
	return func(o *actorOptions) { o.id = id }
}

func WithActorKind(kind string) ActorOption {
	return func(o *actorOptions) { o.kind = kind }
}
