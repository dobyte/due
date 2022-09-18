package router

import (
	"errors"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/registry"
	"sync"
)

var (
	ErrNotFoundRoute    = errors.New("not found route")
	ErrNotFoundEndpoint = errors.New("not found endpoint")
)

type Router struct {
	rw            sync.RWMutex
	routes        map[int32]*Route              // 节点路由表
	gateEndpoints map[string]*endpoint.Endpoint // 网关服务端口
	nodeEndpoints map[string]*endpoint.Endpoint // 节点服务端口
}

func NewRouter() *Router {
	return &Router{
		routes:        make(map[int32]*Route),
		gateEndpoints: make(map[string]*endpoint.Endpoint),
		nodeEndpoints: make(map[string]*endpoint.Endpoint),
	}
}

// ReplaceServices 替换服务实例
func (r *Router) ReplaceServices(services ...*registry.ServiceInstance) {
	r.rw.Lock()
	defer r.rw.Unlock()

	r.routes = make(map[int32]*Route, len(services))
	r.gateEndpoints = make(map[string]*endpoint.Endpoint, len(services))
	r.nodeEndpoints = make(map[string]*endpoint.Endpoint, len(services))

	for _, service := range services {
		_ = r.addService(service)
	}
}

// AddService 添加服务实例
func (r *Router) AddService(service *registry.ServiceInstance) error {
	r.rw.Lock()
	defer r.rw.Unlock()

	return r.addService(service)
}

// RemoveService 移除服务实例
func (r *Router) RemoveService(service *registry.ServiceInstance) {
	r.rw.Lock()
	defer r.rw.Unlock()

	switch service.Name {
	case cluster.Gate.String():
		delete(r.gateEndpoints, service.ID)
	case cluster.Node.String():
		for _, item := range service.Routes {
			if route, ok := r.routes[item.ID]; ok {
				route.endpoints.Delete(service.ID)
			}
		}
	}
}

// 添加服务实例
func (r *Router) addService(service *registry.ServiceInstance) error {
	ep, err := endpoint.ParseEndpoint(service.Endpoint)
	if err != nil {
		return err
	}

	switch service.Name {
	case cluster.Gate.String():
		r.gateEndpoints[service.ID] = ep
	case cluster.Node.String():
		r.nodeEndpoints[service.ID] = ep
		for _, item := range service.Routes {
			route, ok := r.routes[item.ID]
			if !ok {
				route = &Route{
					id:       item.ID,
					stateful: item.Stateful,
				}
				r.routes[item.ID] = route
			}
			route.endpoints.Store(service.ID, ep)
		}
	}

	return nil
}

// FindGateEndpoint 查找网关服务端口
func (r *Router) FindGateEndpoint(insID string) (*Endpoint, error) {
	r.rw.RLock()
	defer r.rw.RUnlock()

	ep, ok := r.gateEndpoints[insID]
	if !ok {
		return nil, ErrNotFoundEndpoint
	}

	return ep, nil
}

// RangeGateEndpoint 轮询网关服务端口
func (r *Router) RangeGateEndpoint(fn func(insID string, ep *Endpoint) bool) {
	r.rw.RLock()
	defer r.rw.RUnlock()

	for insID, ep := range r.gateEndpoints {
		if fn(insID, ep) == false {
			break
		}
	}
}

// FindNodeRoute 查找节点路由
func (r *Router) FindNodeRoute(routeID int32) (*Route, error) {
	r.rw.RLock()
	defer r.rw.RUnlock()

	route, ok := r.routes[routeID]
	if !ok {
		return nil, ErrNotFoundRoute
	}

	return route, nil
}

// FindNodeEndpoint 查找节点服务端口
func (r *Router) FindNodeEndpoint(insID string) (*Endpoint, error) {
	r.rw.RLock()
	defer r.rw.RUnlock()

	ep, ok := r.nodeEndpoints[insID]
	if !ok {
		return nil, ErrNotFoundEndpoint
	}

	return ep, nil
}

// RangeNodeEndpoint 轮询网关服务端口
func (r *Router) RangeNodeEndpoint(fn func(insID string, ep *Endpoint) bool) {
	r.rw.RLock()
	defer r.rw.RUnlock()

	for insID, ep := range r.nodeEndpoints {
		if fn(insID, ep) == false {
			break
		}
	}
}
