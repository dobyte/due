package node

import (
	"context"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xuuid"
	"time"
)

const (
	defaultName    = "node"          // 默认节点名称
	defaultCodec   = "proto"         // 默认编解码器名称
	defaultTimeout = 3 * time.Second // 默认超时时间
)

const (
	defaultIDKey      = "etc.cluster.node.id"
	defaultNameKey    = "etc.cluster.node.name"
	defaultCodecKey   = "etc.cluster.node.codec"
	defaultTimeoutKey = "etc.cluster.node.timeout"
)

const (
	SingleThread SchedulingModel = "single-thread" // 单线程
	MultiThread  SchedulingModel = "multi-thread"  // 多线程
)

// SchedulingModel 调度模型
type SchedulingModel string

type Option func(o *options)

type options struct {
	id              string                // 实例ID
	name            string                // 实例名称；相同实例名称的节点，用户只能绑定其中一个
	ctx             context.Context       // 上下文
	codec           encoding.Codec        // 编解码器
	timeout         time.Duration         // RPC调用超时时间
	locator         locate.Locator        // 用户定位器
	registry        registry.Registry     // 服务注册器
	transporter     transport.Transporter // 消息传输器
	encryptor       crypto.Encryptor      // 消息加密器
	schedulingModel SchedulingModel       // 调度模型
}

func defaultOptions() *options {
	opts := &options{
		ctx:     context.Background(),
		name:    defaultName,
		codec:   encoding.Invoke(defaultCodec),
		timeout: defaultTimeout,
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

	return opts
}

// WithID 设置实例ID
func WithID(id string) Option {
	return func(o *options) { o.id = id }
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

// WithTransporter 设置消息传输器
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) { o.transporter = transporter }
}

// WithEncryptor 设置消息加密器
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) { o.encryptor = encryptor }
}

// WithSchedulingModel 设置调度模型
func WithSchedulingModel(schedulingModel SchedulingModel) Option {
	return func(o *options) { o.schedulingModel = schedulingModel }
}
