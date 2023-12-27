package client

import (
	"context"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xuuid"
	"time"
)

const (
	defaultName    = "client"        // 默认客户端名称
	defaultCodec   = "proto"         // 默认编解码器名称
	defaultTimeout = 3 * time.Second // 默认超时时间
)

const (
	defaultIDKey      = "etc.cluster.client.id"
	defaultNameKey    = "etc.cluster.client.name"
	defaultCodecKey   = "etc.cluster.client.codec"
	defaultTimeoutKey = "etc.cluster.client.timeout"
)

type Option func(o *options)

type options struct {
	id        string           // 实例ID
	name      string           // 实例名称
	ctx       context.Context  // 上下文
	codec     encoding.Codec   // 编解码器
	client    network.Client   // 网络客户端
	timeout   time.Duration    // RPC调用超时时间
	encryptor crypto.Encryptor // 消息加密器
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

	if timeout := etc.Get(defaultTimeoutKey).Int64(); timeout > 0 {
		opts.timeout = time.Duration(timeout) * time.Second
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

// WithClient 设置客户端
func WithClient(client network.Client) Option {
	return func(o *options) { o.client = client }
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithTimeout 设置RPC调用超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithEncryptor 设置消息加密器
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) { o.encryptor = encryptor }
}
