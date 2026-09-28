package dispatcher

import (
	"sync"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/registry"
)

// newTestServiceEndpoint 创建测试服务端点
// @param insID string 实例ID
// @param addr string 端点地址
// @param weight int 权重
// @return @1 *serviceEndpoint 服务端点
func newTestServiceEndpoint(insID, addr string, weight int) *serviceEndpoint {
	return &serviceEndpoint{
		insID:    insID,
		state:    cluster.Work.String(),
		endpoint: endpoint.NewEndpoint("grpc", addr, false),
		weight:   weight,
	}
}

// newWRRTestInstance 创建加权轮询测试服务实例
// @param id string 实例ID
// @param addr string 端点地址
// @param weight int 权重
// @return @1 *registry.ServiceInstance 服务实例
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

	// 权重4:2:1的平滑序列必须精确为 a,b,a,c,a,b,a，禁止出现突发集中
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

	// 100:100:50 经GCD归约为 2:2:1，序列长度控制在5
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

	// 权重0兜底为默认权重1，分布为 1:3
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

	// 原子游标保证每个序列位置恰好分配一次，总选择数为权重和倍数时分布必须精确
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
