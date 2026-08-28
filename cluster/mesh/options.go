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
	defaultName                      = "mesh"  // 默认节点名称
	defaultCodec                     = "proto" // 默认编解码器名称
	defaultWeight                    = 1       // 默认权重
	defaultLinkerConnNum             = 5       // 默认连接数
	defaultLinkerCallTimeout         = "3s"    // 默认调用超时时间
	defaultLinkerDialTimeout         = "3s"    // 默认拨号超时时间
	defaultLinkerDialRetryTimes      = 3       // 默认拨号重试次数
	defaultLinkerFaultRecoveryTime   = "5s"    // 默认故障恢复时间
	defaultLinkerCommandQueueSize    = 4096    // 默认消息队列大小
	defaultLinkerCommandWriteTimeout = "0s"    // 默认写入超时时间
)

const (
	defaultIDKey                        = "etc.cluster.mesh.id"
	defaultNameKey                      = "etc.cluster.mesh.name"
	defaultCodecKey                     = "etc.cluster.mesh.codec"
	defaultWeightKey                    = "etc.cluster.mesh.weight"
	defaultMetadataKey                  = "etc.cluster.mesh.metadata"
	defaultLinkerConnNumKey             = "etc.cluster.mesh.linker.connNum"
	defaultLinkerCallTimeoutKey         = "etc.cluster.mesh.linker.callTimeout"
	defaultLinkerDialTimeoutKey         = "etc.cluster.mesh.linker.dialTimeout"
	defaultLinkerDialRetryTimesKey      = "etc.cluster.mesh.linker.dialRetryTimes"
	defaultLinkerFaultRecoveryTimeKey   = "etc.cluster.mesh.linker.faultRecoveryTime"
	defaultLinkerCommandQueueSizeKey    = "etc.cluster.mesh.linker.commandQueueSize"
	defaultLinkerCommandWriteTimeoutKey = "etc.cluster.mesh.linker.commandWriteTimeout"
)

// Option 微服务配置函数
type Option func(o *options)

// 内部RPC选项
type linkerOptions struct {
	connNum             int           // 内部RPC拨号连接数
	callTimeout         time.Duration // 内部RPC调用超时时间
	dialTimeout         time.Duration // 内部RPC拨号超时时间
	dialRetryTimes      int           // 内部RPC拨号重试次数
	faultRecoveryTime   time.Duration // 内部RPC故障恢复时间
	commandQueueSize    int32         // 消息队列大小
	commandWriteTimeout time.Duration // 消息写入超时时间
}

// 微服务配置项
type options struct {
	id          string                // 实例ID
	name        string                // 实例名称
	ctx         context.Context       // 上下文
	codec       encoding.Codec        // 编解码器
	locator     locate.Locator        // 用户定位器
	registry    registry.Registry     // 服务注册器
	encryptor   crypto.Encryptor      // 消息加密器
	transporter transport.Transporter // 消息传输器
	linker      *linkerOptions        // 连接器配置
	weight      int                   // 服务权重
	metadata    map[string]string     // 元数据
}

// 创建默认微服务配置项
// 从配置环境读取各参数并填充默认值，为未指定的参数生成默认实现
// @return @1 *options 默认微服务配置项
func defaultOptions() *options {
	opts := &options{}
	opts.ctx = context.Background()
	opts.linker = &linkerOptions{}
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

	if weight := etc.Get(defaultWeightKey, defaultWeight).Int(); weight > 0 {
		opts.weight = weight
	} else {
		opts.weight = defaultWeight
	}

	if connNum := etc.Get(defaultLinkerConnNumKey, defaultLinkerConnNum).Int(); connNum > 0 {
		opts.linker.connNum = connNum
	} else {
		opts.linker.connNum = defaultLinkerConnNum
	}

	if callTimeout := etc.Get(defaultLinkerCallTimeoutKey, defaultLinkerCallTimeout).Duration(); callTimeout >= 0 {
		opts.linker.callTimeout = callTimeout
	} else {
		opts.linker.callTimeout = xconv.Duration(defaultLinkerCallTimeout)
	}

	if dialTimeout := etc.Get(defaultLinkerDialTimeoutKey, defaultLinkerDialTimeout).Duration(); dialTimeout >= 0 {
		opts.linker.dialTimeout = dialTimeout
	} else {
		opts.linker.dialTimeout = xconv.Duration(defaultLinkerDialTimeout)
	}

	if dialRetryTimes := etc.Get(defaultLinkerDialRetryTimesKey, defaultLinkerDialRetryTimes).Int(); dialRetryTimes >= 0 {
		opts.linker.dialRetryTimes = dialRetryTimes
	} else {
		opts.linker.dialRetryTimes = defaultLinkerDialRetryTimes
	}

	if faultRecoveryTime := etc.Get(defaultLinkerFaultRecoveryTimeKey, defaultLinkerFaultRecoveryTime).Duration(); faultRecoveryTime >= 0 {
		opts.linker.faultRecoveryTime = faultRecoveryTime
	} else {
		opts.linker.faultRecoveryTime = xconv.Duration(defaultLinkerFaultRecoveryTime)
	}

	if commandQueueSize := etc.Get(defaultLinkerCommandQueueSizeKey, defaultLinkerCommandQueueSize).Int32(); commandQueueSize > 0 {
		opts.linker.commandQueueSize = commandQueueSize
	} else {
		opts.linker.commandQueueSize = defaultLinkerCommandQueueSize
	}

	if commandWriteTimeout := etc.Get(defaultLinkerCommandWriteTimeoutKey, defaultLinkerCommandWriteTimeout).Duration(); commandWriteTimeout >= 0 {
		opts.linker.commandWriteTimeout = commandWriteTimeout
	} else {
		opts.linker.commandWriteTimeout = xconv.Duration(defaultLinkerCommandWriteTimeout)
	}

	if err := etc.Get(defaultMetadataKey).Scan(&opts.metadata); err != nil {
		log.Warnf("scan metadata failed: %v", err)
	}

	return opts
}

// WithID 设置实例ID
// @param id string 实例ID
// @return @1 Option 微服务配置项
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
// @return @1 Option 微服务配置项
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
// @param codec encoding.Codec 编解码器
// @return @1 Option 微服务配置项
func WithCodec(codec encoding.Codec) Option {
	return func(o *options) {
		if codec != nil {
			o.codec = codec
		} else {
			log.Warnf("the specified codec is nil and will be ignored")
		}
	}
}

// WithContext 设置启动上下文
// @param ctx context.Context 启动上下文
// @return @1 Option 微服务配置项
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
// @param locator locate.Locator 用户定位器
// @return @1 Option 微服务配置项
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
// @param r registry.Registry 服务注册器
// @return @1 Option 微服务配置项
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
// @param encryptor crypto.Encryptor 消息加密器
// @return @1 Option 微服务配置项
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
// @param transporter transport.Transporter 消息传输器
// @return @1 Option 微服务配置项
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) {
		if transporter != nil {
			o.transporter = transporter
		} else {
			log.Warnf("the specified transporter is nil and will be ignored")
		}
	}
}

// WithWeight 设置权重
// @param weight int 服务器权重
// @return @1 Option 微服务配置项
func WithWeight(weight int) Option {
	return func(o *options) {
		if weight > 0 {
			o.weight = weight
		} else {
			log.Warnf("the specified weight is less than zero and will be ignored")
		}
	}
}

// WithMetadata 设置元数据
// @param metadata map[string]string 元数据
// @return @1 Option 微服务配置项
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

// WithLinkerConnNum 设置连接数
// @param connNum int 内部RPC拨号连接数
// @return @1 Option 微服务配置项
func WithLinkerConnNum(connNum int) Option {
	return func(o *options) {
		if connNum > 0 {
			o.linker.connNum = connNum
		} else {
			log.Warnf("the specified linker's connNum is less than zero and will be ignored")
		}
	}
}

// WithLinkerCallTimeout 设置RPC调用超时时间
// @param callTimeout time.Duration 内部RPC调用超时时间
// @return @1 Option 微服务配置项
func WithLinkerCallTimeout(callTimeout time.Duration) Option {
	return func(o *options) {
		if callTimeout >= 0 {
			o.linker.callTimeout = callTimeout
		} else {
			log.Warnf("the specified linker's callTimeout is less than zero and will be ignored")
		}
	}
}

// WithLinkerDialTimeout 设置内部RPC拨号超时时间
// @param dialTimeout time.Duration 内部RPC拨号超时时间
// @return @1 Option 微服务配置项
func WithLinkerDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) {
		if dialTimeout >= 0 {
			o.linker.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified linker's dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithLinkerDialRetryTimes 设置内部RPC拨号重试次数
// @param dialRetryTimes int 内部RPC拨号重试次数
// @return @1 Option 微服务配置项
func WithLinkerDialRetryTimes(dialRetryTimes int) Option {
	return func(o *options) {
		if dialRetryTimes >= 0 {
			o.linker.dialRetryTimes = dialRetryTimes
		} else {
			log.Warnf("the specified linker's dialRetryTimes is less than zero and will be ignored")
		}
	}
}

// WithLinkerFaultRecoveryTime 设置内部RPC故障恢复时间
// @param faultRecoveryTime time.Duration 内部RPC故障恢复时间
// @return @1 Option 微服务配置项
func WithLinkerFaultRecoveryTime(faultRecoveryTime time.Duration) Option {
	return func(o *options) {
		if faultRecoveryTime >= 0 {
			o.linker.faultRecoveryTime = faultRecoveryTime
		} else {
			log.Warnf("the specified linker's faultRecoveryTime is less than zero and will be ignored")
		}
	}
}

// WithLinkerCommandQueueSize 设置消息队列大小
// @param commandQueueSize int32 消息队列大小
// @return @1 Option 微服务配置项
func WithLinkerCommandQueueSize(commandQueueSize int32) Option {
	return func(o *options) {
		if commandQueueSize > 0 {
			o.linker.commandQueueSize = commandQueueSize
		} else {
			log.Warnf("the specified linker's messageQueueSize is less than zero and will be ignored")
		}
	}
}

// WithLinkerCommandWriteTimeout 设置写入超时时间
// @param commandWriteTimeout time.Duration 消息写入超时时间
// @return @1 Option 微服务配置项
func WithLinkerCommandWriteTimeout(commandWriteTimeout time.Duration) Option {
	return func(o *options) {
		if commandWriteTimeout >= 0 {
			o.linker.commandWriteTimeout = commandWriteTimeout
		} else {
			log.Warnf("the specified linker's commandWriteTimeout is less than zero and will be ignored")
		}
	}
}
