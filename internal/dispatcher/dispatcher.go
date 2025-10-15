package dispatcher

import (
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
)

type Dispatcher struct {
	dispatch  cluster.Dispatch
	rw        sync.RWMutex
	routes    map[int32]*Route
	events    map[int]*Event
	endpoints map[string]*endpoint.Endpoint
	instances map[string]*registry.ServiceInstance
}

func NewDispatcher(dispatch cluster.Dispatch) *Dispatcher {
	return &Dispatcher{dispatch: dispatch}
}

// FindEndpoint 查找服务端口
func (d *Dispatcher) FindEndpoint(insID string) (*endpoint.Endpoint, error) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	ep, ok := d.endpoints[insID]
	if !ok {
		return nil, errors.ErrNotFoundEndpoint
	}

	return ep, nil
}

// Endpoints 获取所有端口
func (d *Dispatcher) Endpoints() map[string]*endpoint.Endpoint {
	d.rw.RLock()
	defer d.rw.RUnlock()

	return d.endpoints
}

// VisitEndpoints 迭代服务端口
func (d *Dispatcher) VisitEndpoints(fn func(insID string, ep *endpoint.Endpoint) bool) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	for insID, ep := range d.endpoints {
		if !fn(insID, ep) {
			break
		}
	}
}

// FindRoute 查找节点路由
func (d *Dispatcher) FindRoute(route int32) (*Route, error) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	r, ok := d.routes[route]
	if !ok {
		return nil, errors.ErrNotFoundRoute
	}

	return r, nil
}

// FindEvent 查找节点事件
func (d *Dispatcher) FindEvent(event int) (*Event, error) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	e, ok := d.events[event]
	if !ok {
		return nil, errors.ErrNotFoundEvent
	}

	return e, nil
}

// ReplaceServices 替换服务
func (d *Dispatcher) ReplaceServices(services ...*registry.ServiceInstance) {
	routes := make(map[int32]*Route, len(services))
	events := make(map[int]*Event, len(services))
	endpoints := make(map[string]*endpoint.Endpoint)
	instances := make(map[string]*registry.ServiceInstance, len(services))

	for _, service := range services {
		ep, err := endpoint.ParseEndpoint(service.Endpoint)
		if err != nil {
			log.Errorf("service endpoint parse failed, insID: %s kind: %s name: %s alias: %s endpoint: %s err: %v",
				service.ID, service.Kind, service.Name, service.Alias, service.Endpoint, err)
			continue
		}

		endpoints[service.ID] = ep
		instances[service.ID] = service

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

	d.rw.Lock()
	d.routes = routes
	d.events = events
	d.endpoints = endpoints
	d.instances = instances
	d.rw.Unlock()
}
