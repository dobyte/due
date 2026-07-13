package node

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
	defaultName                      = "node"  // 默认节点名称
	defaultCodec                     = "proto" // 默认编解码器名称
	defaultWeight                    = 1       // 默认权重
	defaultAddr                      = ":0"    // 连接器监听地址
	defaultTaskQueueSize             = 4096    // 默认任务队列大小
	defaultTaskWriteTimeout          = "0s"    // 默认任务超时时间
	defaultMessageQueueSize          = 10240   // 默认消息队列大小
	defaultMessageWriteTimeout       = "0s"    // 默认写入超时时间
	defaultLinkerConnNum             = 5       // 默认连接数
	defaultLinkerCallTimeout         = "3s"    // 默认调用超时时间
	defaultLinkerDialTimeout         = "3s"    // 默认拨号超时时间
	defaultLinkerDialRetryTimes      = 3       // 默认拨号重试次数
	defaultLinkerFaultRecoveryTime   = "5s"    // 默认故障恢复时间
	defaultLinkerCommandQueueSize    = 4096    // 默认消息队列大小
	defaultLinkerCommandWriteTimeout = "0s"    // 默认写入超时时间
)

const (
	defaultIDKey                        = "etc.cluster.node.id"
	defaultNameKey                      = "etc.cluster.node.name"
	defaultCodecKey                     = "etc.cluster.node.codec"
	defaultWeightKey                    = "etc.cluster.node.weight"
	defaultMetadataKey                  = "etc.cluster.node.metadata"
	defaultAddrKey                      = "etc.cluster.node.addr"
	defaultExposeKey                    = "etc.cluster.node.expose"
	defaultTaskQueueSizeKey             = "etc.cluster.node.taskQueueSize"
	defaultTaskWriteTimeoutKey          = "etc.cluster.node.taskWriteTimeout"
	defaultMessageQueueSizeKey          = "etc.cluster.node.messageQueueSize"
	defaultMessageWriteTimeoutKey       = "etc.cluster.node.messageWriteTimeout"
	defaultLinkerConnNumKey             = "etc.cluster.node.linker.connNum"
	defaultLinkerCallTimeoutKey         = "etc.cluster.node.linker.callTimeout"
	defaultLinkerDialTimeoutKey         = "etc.cluster.node.linker.dialTimeout"
	defaultLinkerDialRetryTimesKey      = "etc.cluster.node.linker.dialRetryTimes"
	defaultLinkerFaultRecoveryTimeKey   = "etc.cluster.node.linker.faultRecoveryTime"
	defaultLinkerCommandQueueSizeKey    = "etc.cluster.node.linker.commandQueueSize"
	defaultLinkerCommandWriteTimeoutKey = "etc.cluster.node.linker.commandWriteTimeout"
)

type Option func(o *options)

type linkerOptions struct {
	connNum             int           // 内部RPC拨号连接数
	callTimeout         time.Duration // 内部RPC调用超时时间
	dialTimeout         time.Duration // 内部RPC拨号超时时间
	dialRetryTimes      int           // 内部RPC拨号重试次数
	faultRecoveryTime   time.Duration // 内部RPC故障恢复时间
	commandQueueSize    int32         // 消息队列大小
	commandWriteTimeout time.Duration // 消息写入超时时间
}

type options struct {
	ctx                 context.Context        // 启动上下文
	id                  string                 // 实例ID
	name                string                 // 实例名称；相同实例名称的节点，用户只能绑定其中一个
	codec               encoding.Codec         // 编解码器
	weight              int                    // 服务器权重
	locator             locate.Locator         // 用户定位器
	registry            registry.Registry      // 服务注册器
	encryptor           crypto.Encryptor       // 消息加密器
	transporter         transport.Transporter  // 消息传输器
	metadata            map[string]string      // 元数据
	ctxFunc             func() context.Context // 自定义上下文生成器
	addr                string                 // 内部RPC监听地址
	expose              bool                   // 内部RPC是否暴露到公网
	taskQueueSize       int32                  // 任务队列大小
	taskWriteTimeout    time.Duration          // 任务写入超时时间
	messageQueueSize    int32                  // 消息队列大小
	messageWriteTimeout time.Duration          // 消息写入超时时间
	linker              *linkerOptions         // 内部RPC选项
}

func defaultOptions() *options {
	opts := &options{}
	opts.ctx = context.Background()
	opts.expose = etc.Get(defaultExposeKey).Bool()
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

	if addr := etc.Get(defaultAddrKey, defaultAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultAddr
	}

	if taskQueueSize := etc.Get(defaultTaskQueueSizeKey, defaultTaskQueueSize).Int32(); taskQueueSize > 0 {
		opts.taskQueueSize = taskQueueSize
	} else {
		opts.taskQueueSize = defaultTaskQueueSize
	}

	if taskWriteTimeout := etc.Get(defaultTaskWriteTimeoutKey, defaultTaskWriteTimeout).Duration(); taskWriteTimeout >= 0 {
		opts.taskWriteTimeout = taskWriteTimeout
	} else {
		opts.taskWriteTimeout = xconv.Duration(defaultTaskWriteTimeout)
	}

	if messageQueueSize := etc.Get(defaultMessageQueueSizeKey, defaultMessageQueueSize).Int32(); messageQueueSize > 0 {
		opts.messageQueueSize = messageQueueSize
	} else {
		opts.messageQueueSize = defaultMessageQueueSize
	}

	if messageWriteTimeout := etc.Get(defaultMessageWriteTimeoutKey, defaultMessageWriteTimeout).Duration(); messageWriteTimeout >= 0 {
		opts.messageWriteTimeout = messageWriteTimeout
	} else {
		opts.messageWriteTimeout = xconv.Duration(defaultMessageWriteTimeout)
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

// WithAddr 设置监听地址
func WithAddr(addr string) Option {
	return func(o *options) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithExpose 设置是否将内部通信地址暴露到公网
func WithExpose(expose bool) Option {
	return func(o *options) { o.expose = expose }
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

// WithWeight 设置权重
func WithWeight(weight int) Option {
	return func(o *options) {
		if weight > 0 {
			o.weight = weight
		} else {
			log.Warnf("the specified weight is less than zero and will be ignored")
		}
	}
}

// WithContext 设置启动上下文
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

// WithContextFunc 设置自定义上下文生成器
func WithContextFunc(ctxFunc func() context.Context) Option {
	return func(o *options) {
		if ctxFunc != nil {
			o.ctxFunc = ctxFunc
		} else {
			log.Warnf("the specified ctxFunc is nil and will be ignored")
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

// WithTaskQueueSize 设置任务队列大小
func WithTaskQueueSize(taskQueueSize int32) Option {
	return func(o *options) {
		if taskQueueSize > 0 {
			o.taskQueueSize = taskQueueSize
		} else {
			log.Warnf("the specified taskQueueSize is less than zero and will be ignored")
		}
	}
}

// WithTaskWriteTimeout 设置任务超时时间
func WithTaskWriteTimeout(taskWriteTimeout time.Duration) Option {
	return func(o *options) {
		if taskWriteTimeout >= 0 {
			o.taskWriteTimeout = taskWriteTimeout
		} else {
			log.Warnf("the specified taskWriteTimeout is less than zero and will be ignored")
		}
	}
}

// WithMessageWriteTimeout 设置写入超时时间
func WithMessageWriteTimeout(messageWriteTimeout time.Duration) Option {
	return func(o *options) {
		if messageWriteTimeout >= 0 {
			o.messageWriteTimeout = messageWriteTimeout
		} else {
			log.Warnf("the specified messageWriteTimeout is less than zero and will be ignored")
		}
	}
}

// WithMessageQueueSize 设置消息队列大小
func WithMessageQueueSize(messageQueueSize int32) Option {
	return func(o *options) {
		if messageQueueSize > 0 {
			o.messageQueueSize = messageQueueSize
		} else {
			log.Warnf("the specified messageQueueSize is less than zero and will be ignored")
		}
	}
}

// WithLinkerConnNum 设置连接数
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
func WithLinkerCommandWriteTimeout(commandWriteTimeout time.Duration) Option {
	return func(o *options) {
		if commandWriteTimeout >= 0 {
			o.linker.commandWriteTimeout = commandWriteTimeout
		} else {
			log.Warnf("the specified linker's commandWriteTimeout is less than zero and will be ignored")
		}
	}
}
