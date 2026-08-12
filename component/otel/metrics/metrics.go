package metrics

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlphttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/log"
)

var _ component.Component = (*Metrics)(nil)

// Metrics manages the OpenTelemetry metrics provider and OTLP export pipeline.
type Metrics struct {
	component.Base
	opts     *options
	provider *metric.MeterProvider
}

// NewMetrics creates an OpenTelemetry metrics component.
func NewMetrics(opts ...Option) *Metrics {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	if o.endpoint == "" {
		applyProtocolDefault(o)
	}
	return &Metrics{opts: o}
}

// Name returns the component name.
func (*Metrics) Name() string {
	return "metrics"
}

// Init creates and installs the global MeterProvider when metrics are enabled.
func (m *Metrics) Init() {
	if !m.opts.enabled || m.provider != nil {
		return
	}
	if m.opts.protocol != ProtocolGRPC && m.opts.protocol != ProtocolHTTP {
		panic(fmt.Sprintf("unsupported OTLP metrics protocol: %q", m.opts.protocol))
	}

	ctx := context.Background()
	res, err := buildResource(ctx, m.opts)
	if err != nil {
		panic(fmt.Errorf("build OTEL metrics resource: %w", err))
	}

	exportCtx, cancel := context.WithTimeout(ctx, m.opts.timeout)
	exporter, err := buildExporter(exportCtx, m.opts)
	cancel()
	if err != nil {
		panic(fmt.Errorf("build OTLP metrics exporter: %w", err))
	}

	reader := metric.NewPeriodicReader(
		exporter,
		metric.WithInterval(m.opts.interval),
		metric.WithTimeout(m.opts.timeout),
	)
	m.provider = metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)
	otel.SetMeterProvider(m.provider)
}

// Close flushes pending metrics while other components are closing.
func (m *Metrics) Close() {
	if m.provider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.opts.timeout)
	defer cancel()
	if err := m.provider.ForceFlush(ctx); err != nil {
		log.Warnf("OTEL metrics force flush failed: %v", err)
	}
}

// Destroy shuts down the provider and releases the exporter resources.
func (m *Metrics) Destroy() {
	if m.provider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.opts.timeout)
	defer cancel()
	if err := m.provider.Shutdown(ctx); err != nil {
		log.Warnf("OTEL metrics shutdown failed: %v", err)
	}
	m.provider = nil
}

func buildResource(ctx context.Context, opts *options) (*resource.Resource, error) {
	res := resource.DefaultWithContext(ctx)
	var err error
	if opts.resource != nil {
		res, err = resource.Merge(res, opts.resource)
		if err != nil {
			return nil, fmt.Errorf("merge configured resource: %w", err)
		}
	}

	attrs := make([]attribute.KeyValue, 0, len(opts.resourceAttributes)+2)
	attrs = append(attrs, opts.resourceAttributes...)
	if opts.serviceName != "" {
		attrs = append(attrs, attribute.String("service.name", opts.serviceName))
	}
	if opts.serviceInstanceID != "" {
		attrs = append(attrs, attribute.String("service.instance.id", opts.serviceInstanceID))
	}
	if len(attrs) == 0 {
		return res, nil
	}
	return resource.Merge(res, resource.NewSchemaless(attrs...))
}

func buildExporter(ctx context.Context, opts *options) (metric.Exporter, error) {
	switch opts.protocol {
	case ProtocolGRPC:
		exporterOpts := make([]otlpgrpc.Option, 0, 4)
		if strings.Contains(opts.endpoint, "://") {
			exporterOpts = append(exporterOpts, otlpgrpc.WithEndpointURL(opts.endpoint))
		} else {
			exporterOpts = append(exporterOpts, otlpgrpc.WithEndpoint(opts.endpoint))
			if opts.insecure {
				exporterOpts = append(exporterOpts, otlpgrpc.WithInsecure())
			}
		}
		if len(opts.headers) > 0 {
			exporterOpts = append(exporterOpts, otlpgrpc.WithHeaders(opts.headers))
		}
		return otlpgrpc.New(ctx, exporterOpts...)
	case ProtocolHTTP:
		exporterOpts := make([]otlphttp.Option, 0, 4)
		if strings.Contains(opts.endpoint, "://") {
			endpointURL, err := url.Parse(opts.endpoint)
			if err != nil {
				return nil, fmt.Errorf("parse HTTP endpoint URL: %w", err)
			}
			exporterOpts = append(exporterOpts, otlphttp.WithEndpointURL(opts.endpoint))
			if endpointURL.Path == "" {
				exporterOpts = append(exporterOpts, otlphttp.WithURLPath("/v1/metrics"))
			}
		} else {
			exporterOpts = append(exporterOpts, otlphttp.WithEndpoint(opts.endpoint))
			if opts.insecure {
				exporterOpts = append(exporterOpts, otlphttp.WithInsecure())
			}
		}
		if len(opts.headers) > 0 {
			exporterOpts = append(exporterOpts, otlphttp.WithHeaders(opts.headers))
		}
		return otlphttp.New(ctx, exporterOpts...)
	default:
		return nil, fmt.Errorf("unsupported OTLP metrics protocol: %q", opts.protocol)
	}
}
