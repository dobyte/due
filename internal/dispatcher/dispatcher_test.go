package dispatcher_test

import (
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/internal/dispatcher"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/registry"
	"testing"
)

func TestDispatcher_ReplaceServices(t *testing.T) {
	var (
		instance1 = &registry.ServiceInstance{
			ID:       "xc",
			Name:     "gate-3",
			Kind:     cluster.Node,
			Alias:    "gate-3",
			State:    cluster.Work,
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8003", false).String(),
			Routes: []registry.Route{{
				ID:       2,
				Stateful: false,
			}, {
				ID:       3,
				Stateful: false,
			}, {
				ID:       4,
				Stateful: true,
			}},
		}
		instance2 = &registry.ServiceInstance{
			ID:       "xa",
			Name:     "gate-1",
			Kind:     cluster.Node,
			Alias:    "gate-1",
			State:    cluster.Work,
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8001", false).String(),
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}, {
				ID:       2,
				Stateful: false,
			}, {
				ID:       3,
				Stateful: false,
			}, {
				ID:       4,
				Stateful: true,
			}},
		}
		instance3 = &registry.ServiceInstance{
			ID:       "xb",
			Name:     "gate-2",
			Kind:     cluster.Node,
			Alias:    "gate-2",
			State:    cluster.Work,
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8002", false).String(),
			Events:   []cluster.Event{cluster.Disconnect},
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}, {
				ID:       2,
				Stateful: false,
			}},
		}
	)

	d := dispatcher.NewDispatcher(dispatcher.RoundRobin)

	d.ReplaceServices(instance1, instance2, instance3)

	event, err := d.FindEvent(cluster.Disconnect)
	if err != nil {
		t.Errorf("find event failed: %v", err)
	} else {
		t.Log(event.FindEndpoint())
	}
}
