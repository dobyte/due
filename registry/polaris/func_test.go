package polaris

import (
	"slices"
	"testing"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// testInstance is a minimal model.Instance used by the parser tests.
type testInstance struct {
	healthy  bool
	isolated bool
	metadata map[string]string
}

func (i *testInstance) GetInstanceKey() model.InstanceKey { return model.InstanceKey{} }
func (i *testInstance) GetNamespace() string              { return "" }
func (i *testInstance) GetService() string                { return "" }
func (i *testInstance) GetId() string                     { return "" }
func (i *testInstance) GetHost() string                   { return "" }
func (i *testInstance) GetPort() uint32                   { return 0 }
func (i *testInstance) GetVpcId() string                  { return "" }
func (i *testInstance) GetProtocol() string               { return "" }
func (i *testInstance) GetVersion() string                { return "" }
func (i *testInstance) GetWeight() int                    { return 0 }
func (i *testInstance) GetPriority() uint32               { return 0 }
func (i *testInstance) GetMetadata() map[string]string    { return i.metadata }
func (i *testInstance) GetLogicSet() string               { return "" }

func (i *testInstance) GetCircuitBreakerStatus() model.CircuitBreakerStatus { return nil }
func (i *testInstance) IsHealthy() bool                                     { return i.healthy }
func (i *testInstance) IsIsolated() bool                                    { return i.isolated }
func (i *testInstance) IsEnableHealthCheck() bool                           { return false }
func (i *testInstance) GetRegion() string                                   { return "" }
func (i *testInstance) GetZone() string                                     { return "" }
func (i *testInstance) GetIDC() string                                      { return "" }
func (i *testInstance) GetCampus() string                                   { return "" }
func (i *testInstance) GetRevision() string                                 { return "" }
func (i *testInstance) GetTtl() int64                                       { return 0 }
func (i *testInstance) SetHealthy(status bool)                              {}
func (i *testInstance) DeepClone() model.Instance                           { return i }

func TestParseInstancesSkipsInvalidInstances(t *testing.T) {
	instances := []model.Instance{
		&testInstance{healthy: true, metadata: map[string]string{metaFieldID: "bad", metaFieldRoutes: "x:1"}},
		&testInstance{healthy: false, metadata: map[string]string{metaFieldID: "unhealthy", metaFieldRoutes: "x:1"}},
		&testInstance{healthy: true, isolated: true, metadata: map[string]string{metaFieldID: "isolated", metaFieldRoutes: "x:1"}},
		&testInstance{healthy: true, metadata: map[string]string{metaFieldID: "good", metaFieldRoutes: "s:1-2"}},
	}

	services, err := parseInstances(instances)
	if err != nil {
		t.Fatalf("parseInstances() error = %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("parseInstances() returned %d instances, want 1", len(services))
	}

	if services[0].ID != "good" {
		t.Fatalf("parseInstances() ID = %q, want %q", services[0].ID, "good")
	}

	want := []registry.Route{{ID: 1, Stateful: true}, {ID: 2, Stateful: true}}
	if !slices.Equal(services[0].Routes, want) {
		t.Fatalf("parseInstances() routes = %+v, want %+v", services[0].Routes, want)
	}
}

func TestParseInstancesAllInvalid(t *testing.T) {
	instances := []model.Instance{
		&testInstance{healthy: true, metadata: map[string]string{metaFieldID: "bad-1", metaFieldRoutes: "x:1"}},
		&testInstance{healthy: true, metadata: map[string]string{metaFieldID: "bad-2", metaFieldRoutes: "s:3-1"}},
	}

	services, err := parseInstances(instances)
	if !errors.Is(err, errors.ErrAllInstancesParseFailed) {
		t.Fatalf("parseInstances() error = %v, want %v", err, errors.ErrAllInstancesParseFailed)
	}

	if services != nil {
		t.Fatalf("parseInstances() services = %+v, want nil", services)
	}
}

func TestParseInstancesNoHealthy(t *testing.T) {
	instances := []model.Instance{
		&testInstance{metadata: map[string]string{metaFieldID: "unhealthy", metaFieldRoutes: "x:1"}},
	}

	services, err := parseInstances(instances)
	if err != nil {
		t.Fatalf("parseInstances() error = %v", err)
	}

	if len(services) != 0 {
		t.Fatalf("parseInstances() returned %d instances, want 0", len(services))
	}
}

func TestParseInstance(t *testing.T) {
	instance := &testInstance{
		healthy: true,
		metadata: map[string]string{
			metaFieldID:       "node-1",
			metaFieldName:     "node",
			metaFieldKind:     "node",
			metaFieldAlias:    "node-1-alias",
			metaFieldState:    "work",
			metaFieldEndpoint: "grpc://127.0.0.1:3553",
			metaFieldWeight:   "100",
			metaFieldRoutes:   "is:1-2;a:3",
			metaFieldEvents:   `[1,2]`,
			metaFieldServices: `["login"]`,
			metaFieldMetadata: `{"zone":"a"}`,
		},
	}

	ins, err := parseInstance(instance)
	if err != nil {
		t.Fatalf("parseInstance() error = %v", err)
	}

	if ins.ID != "node-1" || ins.Name != "node" || ins.Kind != "node" ||
		ins.Alias != "node-1-alias" || ins.State != "work" ||
		ins.Endpoint != "grpc://127.0.0.1:3553" || ins.Weight != 100 {
		t.Fatalf("parseInstance() = %+v, want the registered instance fields", ins)
	}

	wantRoutes := []registry.Route{
		{ID: 1, Internal: true, Stateful: true},
		{ID: 2, Internal: true, Stateful: true},
		{ID: 3, Authorized: true},
	}
	if !slices.Equal(ins.Routes, wantRoutes) {
		t.Fatalf("parseInstance() routes = %+v, want %+v", ins.Routes, wantRoutes)
	}

	if !slices.Equal(ins.Events, []int{1, 2}) {
		t.Fatalf("parseInstance() events = %+v, want %+v", ins.Events, []int{1, 2})
	}

	if !slices.Equal(ins.Services, []string{"login"}) {
		t.Fatalf("parseInstance() services = %+v, want %+v", ins.Services, []string{"login"})
	}

	if ins.Metadata["zone"] != "a" {
		t.Fatalf("parseInstance() metadata = %+v, want zone=a", ins.Metadata)
	}
}

func TestParseInstanceLegacyRoutes(t *testing.T) {
	ins, err := parseInstance(&testInstance{
		healthy: true,
		metadata: map[string]string{
			metaFieldRoutes: `[{"i":1,"n":true,"s":true},{"i":2,"a":true}]`,
		},
	})
	if err != nil {
		t.Fatalf("parseInstance() error = %v", err)
	}

	want := []registry.Route{
		{ID: 1, Internal: true, Stateful: true},
		{ID: 2, Authorized: true},
	}
	if !slices.Equal(ins.Routes, want) {
		t.Fatalf("parseInstance() routes = %+v, want %+v", ins.Routes, want)
	}
}
