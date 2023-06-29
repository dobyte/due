package dispatcher

import (
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/registry"
	"sync"
)

var (
	ErrNotFoundRoute    = errors.New("not found route")
	ErrNotFoundEvent    = errors.New("not found event")
	ErrNotFoundEndpoint = errors.New("not found endpoint")
)

type BalanceStrategy string

const (
	Random           BalanceStrategy = "random" // 随机
	RoundRobin       BalanceStrategy = "rr"     // 轮询
	WeightRoundRobin BalanceStrategy = "wrr"    // 加权轮询
)

type Dispatcher struct {
	strategy  BalanceStrategy
	rw        sync.RWMutex
	routes    map[int32]*Route
	events    map[cluster.Event]*Event
	endpoints map[string]*endpoint.Endpoint
}

func NewDispatcher(strategy BalanceStrategy) *Dispatcher {
	return &Dispatcher{strategy: strategy}
}

// FindEndpoint 查找服务端口
func (d *Dispatcher) FindEndpoint(insID string) (*endpoint.Endpoint, error) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	ep, ok := d.endpoints[insID]
	if !ok {
		return nil, ErrNotFoundEndpoint
	}

	return ep, nil
}

// IterateEndpoint 迭代服务端口
func (d *Dispatcher) IterateEndpoint(fn func(insID string, ep *endpoint.Endpoint) bool) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	for insID, ep := range d.endpoints {
		if fn(insID, ep) == false {
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
		return nil, ErrNotFoundRoute
	}

	return r, nil
}

// FindEvent 查找节点事件
func (d *Dispatcher) FindEvent(event cluster.Event) (*Event, error) {
	d.rw.RLock()
	defer d.rw.RUnlock()

	r, ok := d.events[event]
	if !ok {
		return nil, ErrNotFoundEvent
	}

	return r, nil
}

// ReplaceServices 替换服务
func (d *Dispatcher) ReplaceServices(services ...*registry.ServiceInstance) {
	routes := make(map[int32]*Route, len(services))
	events := make(map[cluster.Event]*Event, len(services))
	endpoints := make(map[string]*endpoint.Endpoint)

	for _, service := range services {
		ep, err := endpoint.ParseEndpoint(service.Endpoint)
		if err != nil {
			log.Errorf("service endpoint parse failed, insID: %s kind: %s name: %s endpoint: %s err: %v",
				service.ID, service.Kind, service.Name, service.Endpoint, err)
			continue
		}

		endpoints[service.ID] = ep

		for _, item := range service.Routes {
			route, ok := routes[item.ID]
			if !ok {
				route = newRoute(d, item.ID, item.Stateful)
				routes[item.ID] = route
			}
			route.addEndpoint(service.ID, ep)
		}

		for _, evt := range service.Events {
			event, ok := events[evt]
			if !ok {
				event = newEvent(d, evt)
				events[evt] = event
			}
			event.addEndpoint(service.ID, ep)
		}
	}

	d.rw.Lock()
	d.routes = routes
	d.events = events
	d.endpoints = endpoints
	d.rw.Unlock()
}
