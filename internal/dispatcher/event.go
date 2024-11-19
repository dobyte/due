package dispatcher

type Event struct {
	abstract
	event int // 事件ID
}

func newEvent(dispatcher *Dispatcher, event int) *Event {
	return &Event{
		event: event,
		abstract: abstract{
			dispatcher: dispatcher,
			endpoints1: make([]*serviceEndpoint, 0),
			endpoints2: make(map[string]*serviceEndpoint),
			endpoints3: make([]*serviceEndpoint, 0),
			endpoints4: make(map[string]*serviceEndpoint),
		},
	}
}

// Event 获取事件
func (e *Event) Event() int {
	return e.event
}
