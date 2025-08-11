package mesh

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	defaultName    = "mesh"          // 默认节点名称
	defaultCodec   = "proto"         // 默认编解码器名称
	defaultTimeout = 3 * time.Second // 默认超时时间
)

const (
	defaultIDKey       = "etc.cluster.mesh.id"
	defaultNameKey     = "etc.cluster.mesh.name"
	defaultCodecKey    = "etc.cluster.mesh.codec"
	defaultTimeoutKey  = "etc.cluster.mesh.timeout"
	defaultMetadataKey = "etc.cluster.mesh.metadata"
)

type Option func(o *options)

type options struct {
	id          string                // 实例ID
	name        string                // 实例名称
	ctx         context.Context       // 上下文
	codec       encoding.Codec        // 编解码器
	timeout     time.Duration         // RPC调用超时时间
	locator     locate.Locator        // 用户定位器
	registry    registry.Registry     // 服务注册器
	encryptor   crypto.Encryptor      // 消息加密器
	transporter transport.Transporter // 消息传输器
	metadata    map[string]string     // 元数据
}

func defaultOptions() *options {
	opts := &options{
		ctx:      context.Background(),
		name:     defaultName,
		codec:    encoding.Invoke(defaultCodec),
		timeout:  defaultTimeout,
		metadata: make(map[string]string),
	}

	if id := etc.Get(defaultIDKey).String(); id != "" {
		opts.id = id
	} else {
		opts.id = xuuid.UUID()
	}

	if name := etc.Get(defaultNameKey).String(); name != "" {
		opts.name = name
	}

	if codec := etc.Get(defaultCodecKey).String(); codec != "" {
		opts.codec = encoding.Invoke(codec)
	}

	if timeout := etc.Get(defaultTimeoutKey).Duration(); timeout > 0 {
		opts.timeout = timeout
	}

	if err := etc.Get(defaultMetadataKey).Scan(&opts.metadata); err != nil {
		log.Warnf("scan mesh metadata failed: %v", err)
	}

	return opts
}

// WithName 设置实例名称
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithCodec 设置编解码器
func WithCodec(codec encoding.Codec) Option {
	return func(o *options) { o.codec = codec }
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithTimeout 设置RPC调用超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithLocator 设置定位器
func WithLocator(locator locate.Locator) Option {
	return func(o *options) { o.locator = locator }
}

// WithRegistry 设置服务注册器
func WithRegistry(r registry.Registry) Option {
	return func(o *options) { o.registry = r }
}

// WithEncryptor 设置消息加密器
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) { o.encryptor = encryptor }
}

// WithTransporter 设置消息传输器
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) { o.transporter = transporter }
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]string) Option {
	return func(o *options) { o.metadata = metadata }
}
