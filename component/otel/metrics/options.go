package metrics

import (
	"maps"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/dobyte/due/v2/etc"
)

// Protocol selects the OTLP metric exporter transport.
type Protocol string

const (
	ProtocolGRPC Protocol = "grpc"
	ProtocolHTTP Protocol = "http"
)

const (
	defaultServiceName    = "due"            // 默认服务名称
	defaultGRPCEndpoint   = "localhost:4317" // 默认 gRPC 导出端点
	defaultHTTPEndpoint   = "localhost:4318" // 默认 HTTP 导出端点
	defaultExportInterval = 10 * time.Second // 默认指标导出间隔
	defaultExportTimeout  = 3 * time.Second  // 默认单次指标导出超时时间
)
const (
	defaultEnabledKey           = "etc.otel.enable"
	defaultServiceNameKey       = "etc.otel.serviceName"
	defaultServiceInstanceIDKey = "etc.otel.serviceInstanceID"

	defaultEndpointKey       = "etc.otel.metrics.endpoint"
	defaultProtocolKey       = "etc.otel.metrics.protocol"
	defaultInsecureKey       = "etc.otel.metrics.insecure"
	defaultExportIntervalKey = "etc.otel.metrics.exportInterval"
	defaultExportTimeoutKey  = "etc.otel.metrics.exportTimeout"
)

type Option func(o *options)

type options struct {
	// 启用指标导出
	enabled            bool
	serviceName        string
	serviceInstanceID  string
	endpoint           string
	endpointSet        bool
	protocol           Protocol
	insecure           bool
	interval           time.Duration
	timeout            time.Duration
	resource           *resource.Resource
	resourceAttributes []attribute.KeyValue
	headers            map[string]string
}

func defaultOptions() *options {
	opts := &options{
		enabled:     etc.Get(defaultEnabledKey).Bool(),
		serviceName: etc.Get(defaultServiceNameKey, defaultServiceName).String(),
		protocol:    Protocol(etc.Get(defaultProtocolKey, string(ProtocolGRPC)).String()),
		insecure:    etc.Get(defaultInsecureKey, true).Bool(),
		interval:    etc.Get(defaultExportIntervalKey, defaultExportInterval).Duration(),
		timeout:     etc.Get(defaultExportTimeoutKey, defaultExportTimeout).Duration(),
		headers:     make(map[string]string),
	}
	if value := etc.Get(defaultServiceInstanceIDKey).String(); value != "" {
		opts.serviceInstanceID = value
	}
	if value := etc.Get(defaultEndpointKey).String(); value != "" {
		opts.endpoint = value
		opts.endpointSet = true
	}
	applyProtocolDefault(opts)
	return opts
}

func applyProtocolDefault(opts *options) {
	if opts.endpoint != "" {
		return
	}
	if opts.protocol == ProtocolHTTP {
		opts.endpoint = defaultHTTPEndpoint
		return
	}
	opts.endpoint = defaultGRPCEndpoint
}

func WithEnabled(enabled bool) Option {
	return func(o *options) { o.enabled = enabled }
}

func WithServiceName(name string) Option {
	return func(o *options) { o.serviceName = name }
}

func WithServiceInstanceID(id string) Option {
	return func(o *options) { o.serviceInstanceID = id }
}

func WithEndpoint(endpoint string) Option {
	return func(o *options) {
		o.endpoint = endpoint
		o.endpointSet = true
	}
}

func WithProtocol(protocol Protocol) Option {
	return func(o *options) {
		o.protocol = protocol
		if !o.endpointSet {
			o.endpoint = ""
			applyProtocolDefault(o)
		}
	}
}

func WithInsecure(insecure bool) Option {
	return func(o *options) { o.insecure = insecure }
}

func WithExportInterval(interval time.Duration) Option {
	return func(o *options) { o.interval = interval }
}

func WithExportTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

func WithResource(res *resource.Resource) Option {
	return func(o *options) { o.resource = res }
}

func WithResourceAttributes(attrs ...attribute.KeyValue) Option {
	return func(o *options) { o.resourceAttributes = append(o.resourceAttributes, attrs...) }
}

func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		o.headers = maps.Clone(headers)
	}
}
