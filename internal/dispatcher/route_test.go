package dispatcher

import (
	"sync"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/registry"
)

// newTestServiceEndpoint creates a test service endpoint with the given instance ID, address and
// weight.
func newTestServiceEndpoint(insID, addr string, weight int) *serviceEndpoint {
	return &serviceEndpoint{
		insID:    insID,
		state:    cluster.Work.String(),
		endpoint: endpoint.NewEndpoint("grpc", addr, false),
		weight:   weight,
	}
}

// newWRRTestInstance creates a weighted round-robin test service instance with the given ID,
// address and weight.
func newWRRTestInstance(id, addr string, weight int) *registry.ServiceInstance {
	return &registry.ServiceInstance{
		ID:       id,
		Name:     "node-" + id,
		Kind:     cluster.Node.String(),
		Alias:    "node-" + id,
		State:    cluster.Work.String(),
		Weight:   weight,
		Endpoint: endpoint.NewEndpoint("grpc", addr, false).String(),
		Routes:   []registry.Route{{ID: 1}},
	}
}

func TestBuildSmoothWRSequence(t *testing.T) {
	eps := []*serviceEndpoint{
		newTestServiceEndpoint("xa", "127.0.0.1:8001", 4),
		newTestServiceEndpoint("xb", "127.0.0.1:8002", 2),
		newTestServiceEndpoint("xc", "127.0.0.1:8003", 1),
	}

	seq := buildSmoothWRSequence(eps)

	if len(seq) != 7 {
		t.Fatalf("expect sequence length 7, got %d", len(seq))
	}

	// The smooth sequence for weights 4:2:1 must be exactly a,b,a,c,a,b,a with no bursty
	// clustering.
	want := []string{"xa", "xb", "xa", "xc", "xa", "xb", "xa"}
	for i, se := range seq {
		if se.insID != want[i] {
			t.Fatalf("sequence mismatch at %d: expect %s, got %s", i, want[i], se.insID)
		}
	}
}

func TestBuildSmoothWRSequenceGCDReduction(t *testing.T) {
	eps := []*serviceEndpoint{
		newTestServiceEndpoint("xa", "127.0.0.1:8001", 100),
		newTestServiceEndpoint("xb", "127.0.0.1:8002", 100),
		newTestServiceEndpoint("xc", "127.0.0.1:8003", 50),
	}

	seq := buildSmoothWRSequence(eps)

	// 100:100:50 reduces by GCD to 2:2:1, bounding the sequence length to 5.
	if len(seq) != 5 {
		t.Fatalf("expect sequence length 5 after GCD reduction, got %d", len(seq))
	}

	counts := make(map[string]int)
	for _, se := range seq {
		counts[se.insID]++
	}
	if counts["xa"] != 2 || counts["xb"] != 2 || counts["xc"] != 1 {
		t.Fatalf("unexpected distribution: %v", counts)
	}
}

func TestBuildSmoothWRSequenceZeroWeight(t *testing.T) {
	eps := []*serviceEndpoint{
		newTestServiceEndpoint("xa", "127.0.0.1:8001", 0),
		newTestServiceEndpoint("xb", "127.0.0.1:8002", 3),
	}

	seq := buildSmoothWRSequence(eps)

	// A zero weight falls back to the default weight 1, giving a 1:3 distribution.
	if len(seq) != 4 {
		t.Fatalf("expect sequence length 4, got %d", len(seq))
	}

	counts := make(map[string]int)
	for _, se := range seq {
		counts[se.insID]++
	}
	if counts["xa"] != 1 || counts["xb"] != 3 {
		t.Fatalf("unexpected distribution: %v", counts)
	}
}

func TestWeightedRoundRobinDispatchConcurrent(t *testing.T) {
	d := NewDispatcher(cluster.WeightedRoundRobin)
	d.ReplaceServices(
		newWRRTestInstance("xa", "127.0.0.1:8001", 4),
		newWRRTestInstance("xb", "127.0.0.1:8002", 2),
		newWRRTestInstance("xc", "127.0.0.1:8003", 1),
	)

	route, err := d.FindRoute(1)
	if err != nil {
		t.Fatalf("find route failed: %v", err)
	}

	// The atomic cursor selects each sequence position exactly once, so the distribution must be
	// exact when the total selection count is a multiple of the weight sum.
	const total = 70000

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		counts = make(map[string]int)
	)

	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < total/8; i++ {
				ep, err := route.FindEndpoint()
				if err != nil {
					t.Errorf("find endpoint failed: %v", err)
					return
				}

				mu.Lock()
				counts[ep.Address()]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if counts["127.0.0.1:8001"] != 40000 || counts["127.0.0.1:8002"] != 20000 || counts["127.0.0.1:8003"] != 10000 {
		t.Fatalf("distribution must be exact under concurrency, got %v", counts)
	}
}

func TestWeightedRoundRobinDispatchBusyFallback(t *testing.T) {
	d := NewDispatcher(cluster.WeightedRoundRobin)

	instance := newWRRTestInstance("xa", "127.0.0.1:8001", 5)
	instance.State = cluster.Busy.String()

	d.ReplaceServices(instance)

	route, err := d.FindRoute(1)
	if err != nil {
		t.Fatalf("find route failed: %v", err)
	}

	ep, err := route.FindEndpoint()
	if err != nil {
		t.Fatalf("find endpoint failed: %v", err)
	}

	if ep.Address() != "127.0.0.1:8001" {
		t.Fatalf("expect busy endpoint fallback, got %s", ep.Address())
	}
}

func TestWeightedRoundRobinDispatchEmpty(t *testing.T) {
	d := NewDispatcher(cluster.WeightedRoundRobin)
	d.ReplaceServices()

	if _, err := d.FindRoute(1); err == nil {
		t.Fatal("expect error for missing route")
	}
}

func BenchmarkWeightedRoundRobinDispatch(b *testing.B) {
	d := NewDispatcher(cluster.WeightedRoundRobin)
	d.ReplaceServices(
		newWRRTestInstance("xa", "127.0.0.1:8001", 4),
		newWRRTestInstance("xb", "127.0.0.1:8002", 2),
		newWRRTestInstance("xc", "127.0.0.1:8003", 1),
	)

	route, err := d.FindRoute(1)
	if err != nil {
		b.Fatalf("find route failed: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err = route.FindEndpoint(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.ReportAllocs()
}
