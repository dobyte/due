package actor

type Engine struct {
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Spawn(creator Creator, opts ...Option) *Actor {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	act := &Actor{}
	act.opts = o
	act.mailbox = make(chan Context, 4096)
	act.processor = creator(act)
	act.processor = creator(act)
	act.processor.Init()
	//act.dispatch()
	act.processor.Start()

	return act
}
