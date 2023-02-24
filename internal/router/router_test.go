package router_test

import (
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/internal/router"
	"github.com/dobyte/due/registry"
	"testing"
	"time"
)

func TestRouter_ReplaceServices(t *testing.T) {
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
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}, {
				ID:       2,
				Stateful: false,
			}},
		}
	)

	r := router.NewRouter(router.RoundRobin)

	err := r.AddService(instance1)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddService(instance2)
	if err != nil {
		t.Fatal(err)
	}

	err = r.AddService(instance3)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := r.FindServiceEndpoint("xc")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ep.String())
	t.Log()

	go func() {
		time.Sleep(3 * time.Second)
		r.RemoveServices(instance3)
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return
		case <-ticker.C:
			for i := 0; i < 6; i++ {
				route, err := r.FindServiceRoute(2)
				if err != nil {
					t.Fatal(err)
				}

				ep, err := route.FindEndpoint()
				if err != nil {
					t.Fatal(err)
				}

				t.Log(ep.String())
			}
			t.Log()
		}
	}
}
