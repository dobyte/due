package mesh

import (
	"context"
	"maps"
	"time"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	defaultName              = "mesh"  // 默认节点名称
	defaultCodec             = "proto" // 默认编解码器名称
	defaultConnNum           = 5       // 默认连接数
	defaultCallTimeout       = "3s"    // 默认调用超时时间
	defaultDialTimeout       = "3s"    // 默认拨号超时时间
	defaultDialRetryTimes    = 3       // 默认拨号重试次数
	defaultWriteTimeout      = "0s"    // 默认写入超时时间
	defaultWriteQueueSize    = 2048    // 默认写入队列大小
	defaultFaultRecoveryTime = "5s"    // 默认故障恢复时间
)

const (
	defaultIDKey                = "etc.cluster.mesh.id"
	defaultNameKey              = "etc.cluster.mesh.name"
	defaultCodecKey             = "etc.cluster.mesh.codec"
	defaultMetadataKey          = "etc.cluster.mesh.metadata"
	defaultConnNumKey           = "etc.cluster.mesh.connNum"
	defaultCallTimeoutKey       = "etc.cluster.mesh.callTimeout"
	defaultDialTimeoutKey       = "etc.cluster.mesh.dialTimeout"
	defaultDialRetryTimesKey    = "etc.cluster.mesh.dialRetryTimes"
	defaultWriteTimeoutKey      = "etc.cluster.mesh.writeTimeout"
	defaultWriteQueueSizeKey    = "etc.cluster.mesh.writeQueueSize"
	defaultFaultRecoveryTimeKey = "etc.cluster.mesh.faultRecoveryTime"
)

type Option func(o *options)

type options struct {
	id                string                // 实例ID
	name              string                // 实例名称
	ctx               context.Context       // 上下文
	codec             encoding.Codec        // 编解码器
	locator           locate.Locator        // 用户定位器
	registry          registry.Registry     // 服务注册器
	encryptor         crypto.Encryptor      // 消息加密器
	transporter       transport.Transporter // 消息传输器
	metadata          map[string]string     // 元数据
	connNum           int                   // 内部RPC拨号连接数
	callTimeout       time.Duration         // 内部RPC调用超时时间
	dialTimeout       time.Duration         // 内部RPC拨号超时时间
	dialRetryTimes    int                   // 内部RPC拨号重试次数
	writeTimeout      time.Duration         // 内部RPC写入超时时间
	writeQueueSize    int32                 // 内部RPC写入队列大小
	faultRecoveryTime time.Duration         // 内部RPC故障恢复时间
}

func defaultOptions() *options {
	opts := &options{}
	opts.ctx = context.Background()
	opts.metadata = make(map[string]string)

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

	if connNum := etc.Get(defaultConnNumKey, defaultConnNum).Int(); connNum > 0 {
		opts.connNum = connNum
	} else {
		opts.connNum = defaultConnNum
	}

	if callTimeout := etc.Get(defaultCallTimeoutKey, defaultCallTimeout).Duration(); callTimeout >= 0 {
		opts.callTimeout = callTimeout
	} else {
		opts.callTimeout = xconv.Duration(defaultCallTimeout)
	}

	if dialTimeout := etc.Get(defaultDialTimeoutKey, defaultDialTimeout).Duration(); dialTimeout >= 0 {
		opts.dialTimeout = dialTimeout
	} else {
		opts.dialTimeout = xconv.Duration(defaultDialTimeout)
	}

	if dialRetryTimes := etc.Get(defaultDialRetryTimesKey, defaultDialRetryTimes).Int(); dialRetryTimes >= 0 {
		opts.dialRetryTimes = dialRetryTimes
	} else {
		opts.dialRetryTimes = defaultDialRetryTimes
	}

	if writeTimeout := etc.Get(defaultWriteTimeoutKey, defaultWriteTimeout).Duration(); writeTimeout >= 0 {
		opts.writeTimeout = writeTimeout
	} else {
		opts.writeTimeout = xconv.Duration(defaultWriteTimeout)
	}

	if writeQueueSize := etc.Get(defaultWriteQueueSizeKey, defaultWriteQueueSize).Int32(); writeQueueSize > 0 {
		opts.writeQueueSize = writeQueueSize
	} else {
		opts.writeQueueSize = defaultWriteQueueSize
	}

	if faultRecoveryTime := etc.Get(defaultFaultRecoveryTimeKey, defaultFaultRecoveryTime).Duration(); faultRecoveryTime >= 0 {
		opts.faultRecoveryTime = faultRecoveryTime
	} else {
		opts.faultRecoveryTime = xconv.Duration(defaultFaultRecoveryTime)
	}

	if err := etc.Get(defaultMetadataKey).Scan(&opts.metadata); err != nil {
		log.Warnf("scan metadata failed: %v", err)
	}

	return opts
}

// WithID 设置实例ID
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
func WithName(name string) Option {
	return func(o *options) {
		if name != "" {
			o.name = name
		} else {
			log.Warnf("the specified name is empty and will be ignored")
		}
	}
}

// WithCodec 设置编解码器
func WithCodec(codec encoding.Codec) Option {
	return func(o *options) {
		if codec != nil {
			o.codec = codec
		} else {
			log.Warnf("the specified codec is nil and will be ignored")
		}
	}
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		} else {
			log.Warnf("the specified ctx is nil and will be ignored")
		}
	}
}

// WithLocator 设置定位器
func WithLocator(locator locate.Locator) Option {
	return func(o *options) {
		if locator != nil {
			o.locator = locator
		} else {
			log.Warnf("the specified locator is nil and will be ignored")
		}
	}
}

// WithRegistry 设置服务注册器
func WithRegistry(r registry.Registry) Option {
	return func(o *options) {
		if r != nil {
			o.registry = r
		} else {
			log.Warnf("the specified registry is nil and will be ignored")
		}
	}
}

// WithEncryptor 设置消息加密器
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) {
		if encryptor != nil {
			o.encryptor = encryptor
		} else {
			log.Warnf("the specified encryptor is nil and will be ignored")
		}
	}
}

// WithTransporter 设置消息传输器
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) {
		if transporter != nil {
			o.transporter = transporter
		} else {
			log.Warnf("the specified transporter is nil and will be ignored")
		}
	}
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]string) Option {
	return func(o *options) {
		if len(metadata) != 0 {
			if len(o.metadata) == 0 {
				o.metadata = make(map[string]string)
			}

			maps.Copy(o.metadata, metadata)
		} else {
			log.Warnf("the specified metadata is empty and will be ignored")
		}
	}
}

// WithConnNum 设置连接数
func WithConnNum(connNum int) Option {
	return func(o *options) {
		if connNum > 0 {
			o.connNum = connNum
		} else {
			log.Warnf("the specified connNum is less than zero and will be ignored")
		}
	}
}

// WithCallTimeout 设置RPC调用超时时间
func WithCallTimeout(callTimeout time.Duration) Option {
	return func(o *options) {
		if callTimeout >= 0 {
			o.callTimeout = callTimeout
		} else {
			log.Warnf("the specified callTimeout is less than zero and will be ignored")
		}
	}
}

// WithDialTimeout 设置内部RPC拨号超时时间
func WithDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) {
		if dialTimeout >= 0 {
			o.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithDialRetryTimes 设置内部RPC拨号重试次数
func WithDialRetryTimes(dialRetryTimes int) Option {
	return func(o *options) {
		if dialRetryTimes >= 0 {
			o.dialRetryTimes = dialRetryTimes
		} else {
			log.Warnf("the specified dialRetryTimes is less than zero and will be ignored")
		}
	}
}

// WithWriteTimeout 设置内部RPC写入超时时间
func WithWriteTimeout(writeTimeout time.Duration) Option {
	return func(o *options) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithWriteQueueSize 设置内部RPC写入队列大小
func WithWriteQueueSize(writeQueueSize int32) Option {
	return func(o *options) {
		if writeQueueSize >= 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithFaultRecoveryTime 设置内部RPC故障恢复时间
func WithFaultRecoveryTime(faultRecoveryTime time.Duration) Option {
	return func(o *options) {
		if faultRecoveryTime >= 0 {
			o.faultRecoveryTime = faultRecoveryTime
		} else {
			log.Warnf("the specified faultRecoveryTime is less than zero and will be ignored")
		}
	}
}
