package registry

import (
	"context"
)

type Registry interface {
	// Name returns the name of the service registry and discovery component.
	Name() string
	// Register registers a service instance.
	Register(ctx context.Context, ins *ServiceInstance) error
	// Deregister deregisters a service instance.
	Deregister(ctx context.Context, ins *ServiceInstance) error
	// Watch watches for changes to the service instances with the same service name.
	Watch(ctx context.Context, serviceName string) (Watcher, error)
	// Services returns the list of service instances.
	Services(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Close closes the service registry and discovery component.
	Close() error
}

type Discovery interface {
	// Watch watches for changes to the service instances with the same service name.
	Watch(ctx context.Context, serviceName string) (Watcher, error)
	// Services returns the list of service instances.
	Services(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}

type Watcher interface {
	// Next returns the list of service instances.
	Next() ([]*ServiceInstance, error)
	// Stop stops watching.
	Stop() error
}

type ServiceInstance struct {
	// ID is the service entity ID, which is unique for each service entity.
	ID string `json:"id,omitempty"`
	// Name is the name of the service entity.
	Name string `json:"name,omitempty"`
	// Kind is the type of the service entity.
	Kind string `json:"kind,omitempty"`
	// Alias is the alias of the service entity.
	Alias string `json:"alias,omitempty"`
	// State is the state of the service instance.
	State string `json:"state,omitempty"`
	// Events is the set of service events.
	Events []int `json:"events,omitempty"`
	// Routes is the service route IDs.
	Routes []Route `json:"routes,omitempty"`
	// Services is the list of service routes.
	Services []string `json:"services,omitempty"`
	// Endpoint is the exposed port of the microservice entity.
	Endpoint string `json:"endpoint,omitempty"`
	// Weight is the weighted round-robin weight of the microservice route.
	Weight int `json:"weight,omitempty"`
	// Metadata is the metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Route struct {
	// ID is the route ID.
	ID int32 `json:"i,omitempty"`
	// Internal reports whether the route is internal.
	Internal bool `json:"n,omitempty"`
	// Stateful reports whether the route is stateful.
	Stateful bool `json:"s,omitempty"`
	// Authorized reports whether the route is authorized.
	Authorized bool `json:"a,omitempty"`
}
