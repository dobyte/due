package actor

type options struct {
	id   string // ID
	args []any  // 参数
}

type Option func(o *options)

func defaultOptions() *options {
	return &options{}
}

// WithID 设置Actor的ID
func WithID(id string) Option {
	return func(o *options) { o.id = id }
}

// WithArgs 设置Actor的参数
func WithArgs(args ...any) Option {
	return func(o *options) { o.args = append(o.args, args...) }
}
