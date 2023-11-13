package dispatcher

type Event struct {
	abstract
	event int // 事件ID
}

func newEvent(dispatcher *Dispatcher, event int) *Event {
	return &Event{
		abstract: abstract{
			dispatcher:  dispatcher,
			endpointMap: make(map[string]*serviceEndpoint),
			endpointArr: make([]*serviceEndpoint, 0),
		},
		event: event,
	}
}

// Event 获取事件
func (e *Event) Event() int {
	return e.event
}
