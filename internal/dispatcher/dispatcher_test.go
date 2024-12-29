package dispatcher_test

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/internal/dispatcher"
	"github.com/dobyte/due/v2/registry"
	"testing"
	"math"
	"fmt"
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

	d := dispatcher.NewDispatcher(dispatcher.RoundRobin)

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
        // 创建三个服务实例，权重分别为4、2、1
        instance1 = &registry.ServiceInstance{
            ID:       "xa",
            Name:     "gate-1",
            Kind:     cluster.Node.String(),
            Alias:    "gate-1",
            State:    cluster.Work.String(),
            Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8001", false).String(), 
			Weight: 4, // 权重4
            Routes: []registry.Route{{
                ID:       1,
                Stateful: false,
            }},
        }
        instance2 = &registry.ServiceInstance{
            ID:       "xb",
            Name:     "gate-2",
            Kind:     cluster.Node.String(),
            Alias:    "gate-2",
            State:    cluster.Work.String(),
            Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8002", false).String(), 
			Weight: 2, // 权重2
            Routes: []registry.Route{{
                ID:       1,
                Stateful: false,
            }},
        }
        instance3 = &registry.ServiceInstance{
            ID:       "xc",
            Name:     "gate-3",
            Kind:     cluster.Node.String(),
            Alias:    "gate-3",
            State:    cluster.Work.String(),
            Endpoint: endpoint.NewEndpoint("grpc", "127.0.0.1:8003", false).String(),
            Weight: 1, // 权重1
            Routes: []registry.Route{{
                ID:       1,
                Stateful: false,
            }},
        }
    )

    // 创建加权轮询调度器
    d := dispatcher.NewDispatcher(dispatcher.WeightRoundRobin)
    d.ReplaceServices(instance1, instance2, instance3)

    // 统计每个实例被选中的次数
    counts := make(map[string]int)
    totalRounds := 70 // 选择一个能被所有权重和(7)整除的数

    // 执行多轮测试
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

        // 从endpoint中解析实例ID并计数
        parsedEp, err := endpoint.ParseEndpoint(ep.String())
        if err != nil {
            t.Errorf("parse endpoint failed: %v", err)
            return
        }
        addr := parsedEp.Address()
        counts[addr]++
    }

    // 验证分配结果
    expectedRatios := map[string]float64{
        "127.0.0.1:8001": 4.0 / 7.0, // 权重4
        "127.0.0.1:8002": 2.0 / 7.0, // 权重2
        "127.0.0.1:8003": 1.0 / 7.0, // 权重1
    }

    t.Log("Distribution results:")
    for addr, count := range counts {
        ratio := float64(count) / float64(totalRounds)
        expected := expectedRatios[addr]
        t.Logf("Server %s: selected %d times, ratio=%.3f, expected=%.3f",
            addr, count, ratio, expected)
        
        // 验证分配比例是否符合权重比例（允许5%的误差）
        if delta := math.Abs(ratio - expected); delta > 0.05 {
            t.Errorf("distribution ratio for %s is %.3f, want %.3f (±0.05)",
                addr, ratio, expected)
        }
    }

    // 验证总次数
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
        // 创建测试服务实例
        instances = []*registry.ServiceInstance{
            {
                ID:       "xa",
                Name:     "gate-1",
                Kind:     cluster.Node.String(),
                Alias:    "gate-1",
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
                Name:     "gate-2",
                Kind:     cluster.Node.String(),
                Alias:    "gate-2",
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
                Name:     "gate-3",
                Kind:     cluster.Node.String(),
                Alias:    "gate-3",
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

    // 运行不同规模的基准测试
    benchmarks := []struct {
        name          string
        concurrency   int  // 并发数
        instanceCount int  // 服务实例数量
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
            // 准备足够数量的实例
            testInstances := make([]*registry.ServiceInstance, bm.instanceCount)
            for i := 0; i < bm.instanceCount; i++ {
                if i < len(instances) {
                    testInstances[i] = instances[i]
                } else {
                    // 复制最后一个实例并修改ID和端口
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

            // 创建调度器
			d := dispatcher.NewDispatcher(dispatcher.WeightRoundRobin)
			d.ReplaceServices(testInstances...)

            // 重置计时器
            b.ResetTimer()

            // 并发执行基准测试
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

            // 报告内存分配统计
            b.ReportAllocs()
        })
    }
}