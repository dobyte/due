package actor

type options struct {
	id   string // Actor的ID
	kind string // Actor的类型
}

type Option func(o *options)

func defaultOptions() *options {
	return &options{}
}

// WithID 设置Actor的ID
func WithID(id string) Option {
	return func(o *options) { o.id = id }
}

// WithKind 设置Actor的类型
func WithKind(kind string) Option {
	return func(o *options) { o.kind = kind }
}
