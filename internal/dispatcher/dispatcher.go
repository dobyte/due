package dispatcher

import (
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
)

type serviceEndpoint struct {
	insID    string
	state    string
	endpoint *endpoint.Endpoint
	weight   int
}

type Dispatcher struct {
	dispatch  cluster.Dispatch
	routes    atomic.Value
	events    atomic.Value
	endpoints atomic.Value
}

func NewDispatcher(dispatch cluster.Dispatch) *Dispatcher {
	return &Dispatcher{dispatch: dispatch}
}

// FindEndpoint returns the endpoint of the service instance identified by insID.
func (d *Dispatcher) FindEndpoint(insID string) (*endpoint.Endpoint, error) {
	if endpoints, ok := d.endpoints.Load().(map[string]*endpoint.Endpoint); ok {
		if ep, ok := endpoints[insID]; ok {
			return ep, nil
		}
	}

	return nil, errors.ErrNotFoundEndpoint
}

// Endpoints returns all endpoints keyed by instance ID.
func (d *Dispatcher) Endpoints() map[string]*endpoint.Endpoint {
	if endpoints, ok := d.endpoints.Load().(map[string]*endpoint.Endpoint); ok {
		return endpoints
	}

	return nil
}

// VisitEndpoints iterates over all service endpoints, stopping early when fn returns false.
func (d *Dispatcher) VisitEndpoints(fn func(insID string, ep *endpoint.Endpoint) bool) {
	if endpoints, ok := d.endpoints.Load().(map[string]*endpoint.Endpoint); ok {
		for insID, ep := range endpoints {
			if !fn(insID, ep) {
				break
			}
		}
	}
}

// FindRoute returns the node route identified by route.
func (d *Dispatcher) FindRoute(route int32) (*Route, error) {
	if routes, ok := d.routes.Load().(map[int32]*Route); ok {
		if r, ok := routes[route]; ok {
			return r, nil
		}
	}

	return nil, errors.ErrNotFoundRoute
}

// FindEvent returns the node event identified by event.
func (d *Dispatcher) FindEvent(event int) (*Event, error) {
	if events, ok := d.events.Load().(map[int]*Event); ok {
		if e, ok := events[event]; ok {
			return e, nil
		}
	}

	return nil, errors.ErrNotFoundEvent
}

// ReplaceServices rebuilds the routing table from the given service instances. It replaces the
// routes, events and endpoints in use; instances whose endpoint cannot be parsed are skipped.
func (d *Dispatcher) ReplaceServices(services ...*registry.ServiceInstance) {
	routes := make(map[int32]*Route, len(services))
	events := make(map[int]*Event, len(services))
	endpoints := make(map[string]*endpoint.Endpoint)

	for _, service := range services {
		ep, err := endpoint.ParseEndpoint(service.Endpoint)
		if err != nil {
			log.Errorf("service endpoint parse failed, insID: %s kind: %s name: %s alias: %s endpoint: %s err: %v",
				service.ID, service.Kind, service.Name, service.Alias, service.Endpoint, err)
			continue
		}

		endpoints[service.ID] = ep

		for _, item := range service.Routes {
			route, ok := routes[item.ID]
			if !ok {
				route = newRoute(d, service.Alias, item)
				routes[item.ID] = route
			}
			route.addServiceEndpoint(&serviceEndpoint{
				insID:    service.ID,
				state:    service.State,
				endpoint: ep,
				weight:   service.Weight,
			})
		}

		for _, evt := range service.Events {
			event, ok := events[evt]
			if !ok {
				event = newEvent(evt)
				events[evt] = event
			}
			event.addServiceEndpoint(&serviceEndpoint{
				insID:    service.ID,
				state:    service.State,
				endpoint: ep,
				weight:   service.Weight,
			})
		}
	}

	d.routes.Store(routes)
	d.events.Store(events)
	d.endpoints.Store(endpoints)
}
