package client

import (
	"context"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	defaultName  = "client" // 默认客户端名称
	defaultCodec = "proto"  // 默认编解码器名称
)

const (
	defaultIDKey    = "etc.cluster.client.id"
	defaultNameKey  = "etc.cluster.client.name"
	defaultCodecKey = "etc.cluster.client.codec"
)

// Option 客户端配置项
type Option func(o *options)

// options 客户端配置
type options struct {
	id        string           // 实例ID
	name      string           // 实例名称
	ctx       context.Context  // 上下文
	codec     encoding.Codec   // 编解码器
	client    network.Client   // 网络客户端
	encryptor crypto.Encryptor // 消息加密器
}

// defaultOptions 获取默认配置
// 默认配置项从配置中心读取，缺失时使用内置默认值
// @return @1 *options 默认配置
func defaultOptions() *options {
	opts := &options{}
	opts.ctx = context.Background()

	if id := etc.Get(defaultIDKey).String(); id != "" {
		opts.id = id
	} else {
		opts.id = xuuid.UUID()
	}

	if name := etc.Get(defaultNameKey, defaultName).String(); name != "" {
		opts.name = name
	} else {
		opts.name = defaultName
	}

	if codec := etc.Get(defaultCodecKey, defaultCodec).String(); codec != "" {
		opts.codec = encoding.Invoke(codec)
	} else {
		opts.codec = encoding.Invoke(defaultCodec)
	}

	return opts
}

// WithID 设置实例ID
// @param id string 实例ID
// @return @1 Option 客户端配置项
func WithID(id string) Option {
	return func(o *options) {
		if id != "" {
			o.id = id
		} else {
			log.Warnf("the specified id is empty and will be automatically ignored")
		}
	}
}

// WithName 设置实例名称
// @param name string 实例名称
// @return @1 Option 客户端配置项
func WithName(name string) Option {
	return func(o *options) {
		if name != "" {
			o.name = name
		} else {
			log.Warnf("the specified name is empty and will be automatically ignored")
		}
	}
}

// WithCodec 设置编解码器
// @param codec encoding.Codec 编解码器
// @return @1 Option 客户端配置项
func WithCodec(codec encoding.Codec) Option {
	return func(o *options) {
		if codec != nil {
			o.codec = codec
		} else {
			log.Warnf("the specified codec is nil and will be automatically ignored")
		}
	}
}

// WithClient 设置网络客户端
// @param client network.Client 网络客户端
// @return @1 Option 客户端配置项
func WithClient(client network.Client) Option {
	return func(o *options) {
		if client != nil {
			o.client = client
		} else {
			log.Warnf("the specified client is nil and will be automatically ignored")
		}
	}
}

// WithContext 设置上下文
// @param ctx context.Context 上下文
// @return @1 Option 客户端配置项
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		} else {
			log.Warnf("the specified ctx is nil and will be automatically ignored")
		}
	}
}

// WithEncryptor 设置消息加密器
// @param encryptor crypto.Encryptor 消息加密器
// @return @1 Option 客户端配置项
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) {
		if encryptor != nil {
			o.encryptor = encryptor
		} else {
			log.Warnf("the specified encryptor is nil and will be automatically ignored")
		}
	}
}

// DialOption 拨号配置项
type DialOption func(o *dialOptions)

// dialOptions 拨号配置
type dialOptions struct {
	addr  string         // 拨号地址
	attrs map[string]any // 连接属性
}

// WithDialAddr 设置拨号地址
// @param addr string 拨号地址
// @return @1 DialOption 拨号配置项
func WithDialAddr(addr string) DialOption {
	return func(o *dialOptions) { o.addr = addr }
}

// WithConnAttr 设置连接属性
// @param key string 属性键
// @param value any 属性值
// @return @1 DialOption 拨号配置项
func WithConnAttr(key string, value any) DialOption {
	return func(o *dialOptions) { o.attrs[key] = value }
}
