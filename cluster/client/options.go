package client

import (
	"context"
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/crypto"
	"github.com/dobyte/due/encoding"
	"github.com/dobyte/due/network"
	"github.com/dobyte/due/utils/xuuid"
	"time"
)

const (
	defaultName    = "client"        // 默认客户端名称
	defaultCodec   = "proto"         // 默认编解码器名称
	defaultTimeout = 3 * time.Second // 默认超时时间
)

const (
	defaultIDKey        = "config.cluster.client.id"
	defaultNameKey      = "config.cluster.client.name"
	defaultCodecKey     = "config.cluster.client.codec"
	defaultTimeoutKey   = "config.cluster.client.timeout"
	defaultEncryptorKey = "config.cluster.client.encryptor"
	defaultDecryptorKey = "config.cluster.client.decryptor"
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
	decryptor crypto.Decryptor // 消息解密器
}

func defaultOptions() *options {
	opts := &options{
		ctx:     context.Background(),
		name:    defaultName,
		codec:   encoding.Invoke(defaultCodec),
		timeout: defaultTimeout,
	}

	if id := config.Get(defaultIDKey).String(); id != "" {
		opts.id = id
	} else if id, err := xuuid.UUID(); err == nil {
		opts.id = id
	}

	if name := config.Get(defaultNameKey).String(); name != "" {
		opts.name = name
	}

	if codec := config.Get(defaultCodecKey).String(); codec != "" {
		opts.codec = encoding.Invoke(codec)
	}

	if timeout := config.Get(defaultTimeoutKey).Int64(); timeout > 0 {
		opts.timeout = time.Duration(timeout) * time.Second
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

// WithDecryptor 设置消息解密器
func WithDecryptor(decryptor crypto.Decryptor) Option {
	return func(o *options) { o.decryptor = decryptor }
}
