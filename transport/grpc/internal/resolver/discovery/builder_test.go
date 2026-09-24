package discovery

import (
	"slices"
	"testing"

	"github.com/dobyte/due/transport/grpc/v2/internal/balancer/wrr"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/registry"
	"google.golang.org/grpc/resolver"
)

// makeInstance 构造测试用服务实例
func makeInstance(id, state, endpoint string, weight int, services ...string) *registry.ServiceInstance {
	return &registry.ServiceInstance{
		ID:       id,
		Name:     cluster.Mesh.String(),
		State:    state,
		Endpoint: endpoint,
		Weight:   weight,
		Services: services,
	}
}

// stateAddrs 从 resolver.State 中提取地址列表（排序后返回，便于比较）
func stateAddrs(state *resolver.State) []string {
	if state == nil {
		return nil
	}
	addrs := make([]string, 0, len(state.Addresses))
	for _, addr := range state.Addresses {
		addrs = append(addrs, addr.Addr)
	}
	slices.Sort(addrs)
	return addrs
}

// addrWeight 从地址属性中读取权重
func addrWeight(addr resolver.Address) uint32 {
	if addr.Attributes == nil {
		return 0
	}
	v, _ := addr.Attributes.Value(wrr.WeightAttrKey).(uint32)
	return v
}

// snapshotStates 在读锁下拷贝 b.states 的地址列表，便于断言
func snapshotStates(b *Builder) map[string][]string {
	b.rw.RLock()
	result := make(map[string][]string, len(b.states))
	for service, state := range b.states {
		result[service] = stateAddrs(state)
	}
	b.rw.RUnlock()
	return result
}

func TestUpdateStates_PerServicePriority(t *testing.T) {
	tests := []struct {
		name      string
		instances []*registry.ServiceInstance
		want      map[string][]string
	}{
		{
			name: "service A has Work, service B only has Hang: B not stripped",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", 1, "foo"),
				makeInstance("ins-3", cluster.Hang.String(), "grpc://10.0.0.3:9000", 1, "bar"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000"},
				"bar": {"10.0.0.3:9000"},
			},
		},
		{
			name: "same service with Work and Busy: only Work addresses",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", 1, "foo"),
				makeInstance("ins-2", cluster.Busy.String(), "grpc://10.0.0.2:9000", 1, "foo"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000"},
			},
		},
		{
			name: "same service with Work, Busy and Hang: only Work addresses",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", 1, "foo"),
				makeInstance("ins-2", cluster.Busy.String(), "grpc://10.0.0.2:9000", 1, "foo"),
				makeInstance("ins-3", cluster.Hang.String(), "grpc://10.0.0.3:9000", 1, "foo"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000"},
			},
		},
		{
			name: "no Work in cluster: each service gets own tier",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Busy.String(), "grpc://10.0.0.1:9000", 1, "foo"),
				makeInstance("ins-2", cluster.Hang.String(), "grpc://10.0.0.3:9000", 1, "bar"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000"},
				"bar": {"10.0.0.3:9000"},
			},
		},
		{
			name:      "empty instances: empty map",
			instances: nil,
			want:      map[string][]string{},
		},
		{
			name: "Shut state instances are ignored",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Shut.String(), "grpc://10.0.0.1:9000", 1, "foo"),
			},
			want: map[string][]string{},
		},
		{
			name: "instance providing multiple services",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", 1, "foo", "bar"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000"},
				"bar": {"10.0.0.1:9000"},
			},
		},
		{
			name: "multiple Work instances for same service",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", 1, "foo"),
				makeInstance("ins-2", cluster.Work.String(), "grpc://10.0.0.2:9000", 1, "foo"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000", "10.0.0.2:9000"},
			},
		},
		{
			name: "service A has Work+Busy, service B only Busy: A gets Work, B gets Busy",
			instances: []*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", 1, "foo"),
				makeInstance("ins-2", cluster.Busy.String(), "grpc://10.0.0.2:9000", 1, "foo"),
				makeInstance("ins-3", cluster.Busy.String(), "grpc://10.0.0.3:9000", 1, "bar"),
			},
			want: map[string][]string{
				"foo": {"10.0.0.1:9000"},
				"bar": {"10.0.0.3:9000"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			b.UpdateStates(tt.instances)

			got := snapshotStates(b)

			if len(got) != len(tt.want) {
				t.Fatalf("service count mismatch: got %d, want %d (got=%v, want=%v)",
					len(got), len(tt.want), got, tt.want)
			}

			for service, wantAddrs := range tt.want {
				gotAddrs, ok := got[service]
				if !ok {
					t.Errorf("service %q missing from states", service)
					continue
				}
				if !slices.Equal(gotAddrs, wantAddrs) {
					t.Errorf("service %q addrs mismatch: got %v, want %v",
						service, gotAddrs, wantAddrs)
				}
			}
		})
	}
}

func TestUpdateStates_WeightPreserved(t *testing.T) {
	tests := []struct {
		name       string
		weight     int
		wantWeight uint32
	}{
		{"explicit weight 5", 5, 5},
		{"zero weight defaults to 1", 0, 1},
		{"negative weight defaults to 1", -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			b.UpdateStates([]*registry.ServiceInstance{
				makeInstance("ins-1", cluster.Work.String(), "grpc://10.0.0.1:9000", tt.weight, "foo"),
			})

			b.rw.RLock()
			state := b.states["foo"]
			b.rw.RUnlock()

			if state == nil {
				t.Fatal("service foo missing from states")
			}
			if len(state.Addresses) != 1 {
				t.Fatalf("expected 1 address, got %d", len(state.Addresses))
			}
			if w := addrWeight(state.Addresses[0]); w != tt.wantWeight {
				t.Errorf("weight mismatch: got %d, want %d", w, tt.wantWeight)
			}
		})
	}
}
