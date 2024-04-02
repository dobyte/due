/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/7 1:31 上午
 * @Desc: TODO
 */

package gate

import (
	"context"
	"github.com/symsimmy/due/config"
	"github.com/symsimmy/due/crypto"
	"github.com/symsimmy/due/encoding"
	"github.com/symsimmy/due/hook"
	"github.com/symsimmy/due/locate"
	"github.com/symsimmy/due/metrics/cat"
	"github.com/symsimmy/due/metrics/prometheus"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/utils/xuuid"
	"time"

	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/registry"
)

const (
	defaultName    = "gate"           // 默认名称
	defaultTimeout = 30 * time.Second // 默认超时时间
	defaultCodec   = "proto"          // 默认编解码器名称
)

const (
	defaultIDKey        = "config.cluster.gate.id"
	defaultNameKey      = "config.cluster.gate.name"
	defaultTimeoutKey   = "config.cluster.gate.timeout"
	defaultCodecKey     = "config.cluster.gate.codec"
	defaultDecryptorKey = "config.cluster.gate.decryptor"
	defaultEncryptorKey = "config.cluster.node.encryptor"
)

type Option func(o *options)

type options struct {
	id          string                // 实例ID
	name        string                // 实例名称
	ctx         context.Context       // 上下文
	timeout     time.Duration         // RPC调用超时时间
	server      network.Server        // 网关服务器
	locator     locate.Locator        // 用户定位器
	registry    registry.Registry     // 服务注册器
	codec       encoding.Codec        // 编解码器
	transporter transport.Transporter // 消息传输器
	promServer  prometheus.PromServer // 埋点采集服务器
	catServer   *cat.Server
	receiveHook []hook.ReceiveHook // 接受消息hook
	encryptor   crypto.Encryptor   // 消息加密器
	decryptor   crypto.Decryptor
}

func defaultOptions() *options {
	opts := &options{
		ctx:     context.Background(),
		name:    defaultName,
		timeout: defaultTimeout,
		codec:   encoding.Invoke(defaultCodec),
	}

	if id := config.Get(defaultIDKey).String(); id != "" {
		opts.id = id
	} else if id, err := xuuid.UUID(); err == nil {
		opts.id = id
	}

	if name := config.Get(defaultNameKey).String(); name != "" {
		opts.name = name
	}

	if timeout := config.Get(defaultTimeoutKey).Int64(); timeout > 0 {
		opts.timeout = time.Duration(timeout) * time.Second
	}

	if codec := config.Get(defaultCodecKey).String(); codec != "" {
		opts.codec = encoding.Invoke(codec)
	}

	if encryptor := config.Get(defaultEncryptorKey).String(); encryptor != "" {
		opts.encryptor = crypto.InvokeEncryptor(encryptor)
	}

	if decryptor := config.Get(defaultDecryptorKey).String(); decryptor != "" {
		opts.decryptor = crypto.InvokeDecryptor(decryptor)
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

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithServer 设置服务器
func WithServer(server network.Server) Option {
	return func(o *options) { o.server = server }
}

// WithTimeout 设置RPC调用超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithLocator 设置用户定位器
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

// WithPromServer 设置prom server
func WithPromServer(promServer *prometheus.PromServer) Option {
	return func(o *options) { o.promServer = *promServer }
}

// WithCatServer 设置cat server
func WithCatServer(catServer *cat.Server) Option {
	return func(o *options) { o.catServer = catServer }
}

// WithReceiveHook 设置Gate收到消息时的Hook函数
func WithReceiveHook(receiveHook ...hook.ReceiveHook) Option {
	return func(o *options) { o.receiveHook = append(o.receiveHook, receiveHook...) }
}
