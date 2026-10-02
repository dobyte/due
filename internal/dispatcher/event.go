package dispatcher

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
)

type Event struct {
	eps   map[string]*serviceEndpoint // All endpoints, including instances in the work, busy and hang states
	event int                         // Event ID
}

func newEvent(event int) *Event {
	return &Event{
		eps:   make(map[string]*serviceEndpoint),
		event: event,
	}
}

// Event returns the event ID.
func (e *Event) Event() int {
	return e.event
}

// VisitEndpoints iterates over the service endpoints, stopping early when fn returns false.
func (e *Event) VisitEndpoints(fn func(insID string, ep *endpoint.Endpoint) bool) {
	for insID, se := range e.eps {
		if !fn(insID, se.endpoint) {
			return
		}
	}
}

// addServiceEndpoint adds a service endpoint unless the instance has been shut down.
func (e *Event) addServiceEndpoint(se *serviceEndpoint) {
	if se.state != cluster.Shut.String() {
		e.eps[se.insID] = se
	}
}
