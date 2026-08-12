package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestDefaultOptionsDisabled(t *testing.T) {
	opts := defaultOptions()

	if opts.enabled {
		t.Fatal("metrics must be disabled by default")
	}
	if opts.serviceName != defaultServiceName {
		t.Fatalf("service name = %q, want %q", opts.serviceName, defaultServiceName)
	}
	if opts.protocol != ProtocolGRPC {
		t.Fatalf("protocol = %q, want %q", opts.protocol, ProtocolGRPC)
	}
	if opts.endpoint != defaultGRPCEndpoint {
		t.Fatalf("endpoint = %q, want %q", opts.endpoint, defaultGRPCEndpoint)
	}
}

func TestMetricsName(t *testing.T) {
	if got := NewMetrics().Name(); got != "metrics" {
		t.Fatalf("name = %q, want metrics", got)
	}
}

func TestOptionsOverrideDefaults(t *testing.T) {
	opts := defaultOptions()
	WithEnabled(true)(opts)
	WithServiceName("game-node")(opts)
	WithServiceInstanceID("node-1")(opts)
	WithProtocol(ProtocolHTTP)(opts)
	WithEndpoint("collector:4318")(opts)
	WithInsecure(false)(opts)
	WithExportInterval(2 * time.Second)(opts)
	WithExportTimeout(1 * time.Second)(opts)

	if !opts.enabled || opts.serviceName != "game-node" || opts.serviceInstanceID != "node-1" {
		t.Fatalf("identity options were not applied: %+v", opts)
	}
	if opts.protocol != ProtocolHTTP || opts.endpoint != "collector:4318" || opts.insecure {
		t.Fatalf("transport options were not applied: %+v", opts)
	}
	if opts.interval != 2*time.Second || opts.timeout != time.Second {
		t.Fatalf("timing options were not applied: %+v", opts)
	}
}

func TestEndpointDefaultsFollowProtocol(t *testing.T) {
	opts := defaultOptions()
	opts.endpointSet = false
	opts.endpoint = ""
	WithProtocol(ProtocolHTTP)(opts)
	applyProtocolDefault(opts)

	if opts.endpoint != defaultHTTPEndpoint {
		t.Fatalf("HTTP endpoint = %q, want %q", opts.endpoint, defaultHTTPEndpoint)
	}
}

func TestExplicitEndpointHasPriorityRegardlessOfOptionOrder(t *testing.T) {
	for _, apply := range []func(*options){
		func(opts *options) {
			WithEndpoint("collector:4317")(opts)
			WithProtocol(ProtocolHTTP)(opts)
		},
		func(opts *options) {
			WithProtocol(ProtocolHTTP)(opts)
			WithEndpoint("collector:4317")(opts)
		},
	} {
		opts := defaultOptions()
		apply(opts)
		if opts.endpoint != "collector:4317" {
			t.Fatalf("endpoint = %q, want explicit endpoint", opts.endpoint)
		}
	}
}

func TestBuildResourceDueAttributesOverrideDefaults(t *testing.T) {
	base := resource.NewWithAttributes("", attribute.String("service.name", "environment-name"))
	opts := defaultOptions()
	opts.serviceName = "due-service"
	opts.serviceInstanceID = "instance-1"
	opts.resource = base
	opts.resourceAttributes = []attribute.KeyValue{
		attribute.String("deployment.environment.name", "test"),
	}

	res, err := buildResource(context.Background(), opts)
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}

	attrs := make(map[string]any)
	for _, kv := range res.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsInterface()
	}
	if attrs["service.name"] != "due-service" {
		t.Fatalf("service.name = %v, want due-service", attrs["service.name"])
	}
	if attrs["service.instance.id"] != "instance-1" {
		t.Fatalf("service.instance.id = %v, want instance-1", attrs["service.instance.id"])
	}
	if attrs["deployment.environment.name"] != "test" {
		t.Fatalf("deployment.environment.name = %v, want test", attrs["deployment.environment.name"])
	}
}

func TestDisabledMetricsDoesNotReplaceGlobalProvider(t *testing.T) {
	provider := otel.GetMeterProvider()
	metrics := NewMetrics(WithProtocol(Protocol("invalid")))
	metrics.Init()

	if otel.GetMeterProvider() != provider {
		t.Fatal("disabled metrics replaced the global meter provider")
	}
}

func TestBuildExporterProtocols(t *testing.T) {
	for _, test := range []struct {
		name     string
		protocol Protocol
		endpoint string
	}{
		{name: "grpc", protocol: ProtocolGRPC, endpoint: defaultGRPCEndpoint},
		{name: "http", protocol: ProtocolHTTP, endpoint: defaultHTTPEndpoint},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := defaultOptions()
			opts.protocol = test.protocol
			opts.endpoint = test.endpoint
			opts.insecure = true

			exporter, err := buildExporter(context.Background(), opts)
			if err != nil {
				t.Fatalf("build %s exporter: %v", test.name, err)
			}
			if err := exporter.Shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown %s exporter: %v", test.name, err)
			}
		})
	}
}

func TestHTTPExporterURLPath(t *testing.T) {
	paths := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	for _, test := range []struct {
		name     string
		suffix   string
		wantPath string
	}{
		{name: "default metrics path", wantPath: "/v1/metrics"},
		{name: "explicit path", suffix: "/custom/metrics", wantPath: "/custom/metrics"},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := defaultOptions()
			opts.protocol = ProtocolHTTP
			opts.endpoint = server.URL + test.suffix

			exporter, err := buildExporter(context.Background(), opts)
			if err != nil {
				t.Fatalf("build HTTP exporter: %v", err)
			}
			t.Cleanup(func() {
				if err := exporter.Shutdown(context.Background()); err != nil {
					t.Errorf("shutdown HTTP exporter: %v", err)
				}
			})

			if err := exporter.Export(context.Background(), &metricdata.ResourceMetrics{}); err != nil {
				t.Fatalf("export metrics: %v", err)
			}
			if got := <-paths; got != test.wantPath {
				t.Fatalf("request path = %q, want %q", got, test.wantPath)
			}
		})
	}
}

func TestEnabledMetricsLifecycle(t *testing.T) {
	m := NewMetrics(
		WithEnabled(true),
		WithProtocol(ProtocolHTTP),
		WithEndpoint(defaultHTTPEndpoint),
		WithInsecure(true),
		WithExportInterval(time.Hour),
		WithExportTimeout(time.Second),
	)
	m.Init()
	if m.provider == nil {
		t.Fatal("enabled metrics did not create a provider")
	}
	provider := m.provider
	m.Init()
	if m.provider != provider {
		t.Fatal("repeated init replaced the active provider")
	}
	if otel.GetMeterProvider() != m.provider {
		t.Fatal("enabled metrics did not install its provider globally")
	}

	m.Close()
	m.Destroy()
	m.Destroy()
	if m.provider != nil {
		t.Fatal("destroy did not release the provider")
	}
}
