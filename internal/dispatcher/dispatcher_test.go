package dispatcher_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/internal/dispatcher"
	"github.com/dobyte/due/v2/registry"
)

func TestDispatcher_ReplaceServices(t *testing.T) {
	var (
		instance1 = &registry.ServiceInstance{
			ID:       "xc",
			Name:     "gate-3",
			Kind:     cluster.Node.String(),
			Alias:    "gate-3",
			State:    cluster.Work.String(),
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
			Kind:     cluster.Node.String(),
			Alias:    "gate-1",
			State:    cluster.Work.String(),
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
			Kind:     cluster.Node.String(),
			Alias:    "gate-2",
			State:    cluster.Hang.String(),
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8002", false).String(),
			Events:   []int{int(cluster.Disconnect)},
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}, {
				ID:       2,
				Stateful: false,
			}},
		}
	)

	d := dispatcher.NewDispatcher(cluster.WeightedRoundRobin)

	d.ReplaceServices(instance1, instance2, instance3)

	route, err := d.FindRoute(1)
	if err != nil {
		t.Errorf("find event failed: %v", err)
	} else {
		t.Log(route.FindEndpoint())
	}

	//event, err := d.FindEvent(int(cluster.Disconnect))
	//if err != nil {
	//	t.Errorf("find event failed: %v", err)
	//} else {
	//	t.Log(event.FindEndpoint())
	//}
}

func TestDispatcher_WeightRoundRobin(t *testing.T) {
	var (
		// Create three service instances with weights 4, 2 and 1.
		instance1 = &registry.ServiceInstance{
			ID:       "xa",
			Name:     "node-1",
			Kind:     cluster.Node.String(),
			Alias:    "node-1",
			State:    cluster.Work.String(),
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8001", false).String(),
			Weight:   4, // Weight 4
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}},
		}
		instance2 = &registry.ServiceInstance{
			ID:       "xb",
			Name:     "node-2",
			Kind:     cluster.Node.String(),
			Alias:    "node-2",
			State:    cluster.Work.String(),
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8002", false).String(),
			Weight:   2, // Weight 2
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}},
		}
		instance3 = &registry.ServiceInstance{
			ID:       "xc",
			Name:     "node-3",
			Kind:     cluster.Node.String(),
			Alias:    "node-3",
			State:    cluster.Work.String(),
			Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8003", false).String(),
			Weight:   1, // Weight 1
			Routes: []registry.Route{{
				ID:       1,
				Stateful: false,
			}},
		}
	)

	// Create a weighted round-robin dispatcher.
	d := dispatcher.NewDispatcher(cluster.WeightedRoundRobin)
	d.ReplaceServices(instance1, instance2, instance3)

	// Count how many times each instance is selected.
	counts := make(map[string]int)
	totalRounds := 200 // 200 is divisible by the sum of all weights (7)

	// Run multiple rounds.
	for i := 0; i < totalRounds; i++ {
		route, err := d.FindRoute(1)
		if err != nil {
			t.Errorf("find route failed: %v", err)
			return
		}

		ep, err := route.FindEndpoint()
		if err != nil {
			t.Errorf("find endpoint failed: %v", err)
			return
		}

		// Parse the instance address from the endpoint and count it.
		parsedEp, err := endpoint.ParseEndpoint(ep.String())
		if err != nil {
			t.Errorf("parse endpoint failed: %v", err)
			return
		}
		addr := parsedEp.Address()
		counts[addr]++
	}

	// Verify the distribution.
	expectedRatios := map[string]float64{
		"127.0.0.1:8001": 4.0 / 7.0, // Weight 4
		"127.0.0.1:8002": 2.0 / 7.0, // Weight 2
		"127.0.0.1:8003": 1.0 / 7.0, // Weight 1
	}

	t.Log("Distribution results:")
	for addr, count := range counts {
		ratio := float64(count) / float64(totalRounds)
		expected := expectedRatios[addr]
		t.Logf("Server %s: selected %d times, ratio=%.3f, expected=%.3f",
			addr, count, ratio, expected)

		// Verify that the ratio matches the weight ratio (allowing a 5% error).
		if delta := math.Abs(ratio - expected); delta > 0.05 {
			t.Errorf("distribution ratio for %s is %.3f, want %.3f (±0.05)",
				addr, ratio, expected)
		}
	}

	// Verify the total count.
	total := 0
	for _, count := range counts {
		total += count
	}
	if total != totalRounds {
		t.Errorf("total rounds = %d, want %d", total, totalRounds)
	}
}

func BenchmarkDispatcher_WeightRoundRobin(b *testing.B) {
	var (
		// Create the test service instances.
		instances = []*registry.ServiceInstance{
			{
				ID:       "xa",
				Name:     "node-1",
				Kind:     cluster.Node.String(),
				Alias:    "node-1",
				State:    cluster.Work.String(),
				Weight:   4,
				Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8001", false).String(),
				Routes: []registry.Route{{
					ID:       1,
					Stateful: false,
				}},
			},
			{
				ID:       "xb",
				Name:     "node-2",
				Kind:     cluster.Node.String(),
				Alias:    "node-2",
				State:    cluster.Work.String(),
				Weight:   2,
				Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8002", false).String(),
				Routes: []registry.Route{{
					ID:       1,
					Stateful: false,
				}},
			},
			{
				ID:       "xc",
				Name:     "node-3",
				Kind:     cluster.Node.String(),
				Alias:    "node-3",
				State:    cluster.Work.String(),
				Weight:   1,
				Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8003", false).String(),
				Routes: []registry.Route{{
					ID:       1,
					Stateful: false,
				}},
			},
		}
	)

	// Run benchmarks at different scales.
	benchmarks := []struct {
		name          string
		concurrency   int // Number of concurrent goroutines
		instanceCount int // Number of service instances
	}{
		{"Concurrency1_Instances3", 1, 3},
		{"Concurrency10_Instances3", 10, 3},
		{"Concurrency100_Instances3", 100, 3},
		{"Concurrency1_Instances10", 1, 10},
		{"Concurrency10_Instances10", 10, 10},
		{"Concurrency100_Instances10", 100, 10},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Prepare enough instances.
			testInstances := make([]*registry.ServiceInstance, bm.instanceCount)
			for i := 0; i < bm.instanceCount; i++ {
				if i < len(instances) {
					testInstances[i] = instances[i]
				} else {
					// Copy the last instance and change its ID and port.
					last := instances[len(instances)-1]
					testInstances[i] = &registry.ServiceInstance{
						ID:       fmt.Sprintf("x%d", i),
						Name:     fmt.Sprintf("gate-%d", i+1),
						Kind:     last.Kind,
						Alias:    fmt.Sprintf("gate-%d", i+1),
						State:    last.State,
						Weight:   1,
						Endpoint: endpoint.NewEndpoint("grpc", fmt.Sprintf("127.0.0.1:%d", 8000+i), false).String(),
						Routes:   last.Routes,
					}
				}
			}

			// Create the dispatcher.
			d := dispatcher.NewDispatcher(cluster.WeightedRoundRobin)
			d.ReplaceServices(testInstances...)

			// Reset the timer.
			b.ResetTimer()

			// Run the benchmark concurrently.
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					route, err := d.FindRoute(1)
					if err != nil {
						b.Fatal(err)
					}
					_, err = route.FindEndpoint()
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			// Report memory allocation statistics.
			b.ReportAllocs()
		})
	}
}
