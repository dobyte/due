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
	defaultName                      = "mesh"  // default node name
	defaultCodec                     = "proto" // default codec name
	defaultWeight                    = 1       // default weight
	defaultLinkerConnNum             = 5       // default number of connections
	defaultLinkerCallTimeout         = "3s"    // default call timeout
	defaultLinkerDialTimeout         = "3s"    // default dial timeout
	defaultLinkerDialRetryTimes      = 3       // default number of dial retries
	defaultLinkerFaultRecoveryTime   = "5s"    // default fault recovery time
	defaultLinkerCommandQueueSize    = 4096    // default message queue size
	defaultLinkerCommandWriteTimeout = "0s"    // default write timeout
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

// Option is a mesh option.
type Option func(o *options)

// linkerOptions are the internal RPC options.
type linkerOptions struct {
	connNum             int           // number of internal RPC dial connections
	callTimeout         time.Duration // internal RPC call timeout
	dialTimeout         time.Duration // internal RPC dial timeout
	dialRetryTimes      int           // number of internal RPC dial retries
	faultRecoveryTime   time.Duration // internal RPC fault recovery time
	commandQueueSize    int32         // message queue size
	commandWriteTimeout time.Duration // message write timeout
}

// options are the mesh options.
type options struct {
	id          string                // instance ID
	name        string                // instance name
	ctx         context.Context       // context
	codec       encoding.Codec        // codec
	locator     locate.Locator        // user locator
	registry    registry.Registry     // service registry
	encryptor   crypto.Encryptor      // message encryptor
	transporter transport.Transporter // message transporter
	linker      *linkerOptions        // linker options
	weight      int                   // service weight
	metadata    map[string]string     // metadata
}

// defaultOptions returns the default mesh options.
//
// It reads each parameter from the configuration environment and fills in the defaults,
// generating default implementations for the unspecified parameters.
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

// WithID sets the instance ID.
func WithID(id string) Option {
	return func(o *options) {
		if id != "" {
			o.id = id
		} else {
			log.Warnf("the specified id is empty and will be automatically ignored")
		}
	}
}

// WithName sets the instance name.
func WithName(name string) Option {
	return func(o *options) {
		if name != "" {
			o.name = name
		} else {
			log.Warnf("the specified name is empty and will be ignored")
		}
	}
}

// WithCodec sets the codec.
func WithCodec(codec encoding.Codec) Option {
	return func(o *options) {
		if codec != nil {
			o.codec = codec
		} else {
			log.Warnf("the specified codec is nil and will be ignored")
		}
	}
}

// WithContext sets the startup context.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		} else {
			log.Warnf("the specified ctx is nil and will be ignored")
		}
	}
}

// WithLocator sets the locator.
func WithLocator(locator locate.Locator) Option {
	return func(o *options) {
		if locator != nil {
			o.locator = locator
		} else {
			log.Warnf("the specified locator is nil and will be ignored")
		}
	}
}

// WithRegistry sets the service registry.
func WithRegistry(r registry.Registry) Option {
	return func(o *options) {
		if r != nil {
			o.registry = r
		} else {
			log.Warnf("the specified registry is nil and will be ignored")
		}
	}
}

// WithEncryptor sets the message encryptor.
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) {
		if encryptor != nil {
			o.encryptor = encryptor
		} else {
			log.Warnf("the specified encryptor is nil and will be ignored")
		}
	}
}

// WithTransporter sets the message transporter.
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) {
		if transporter != nil {
			o.transporter = transporter
		} else {
			log.Warnf("the specified transporter is nil and will be ignored")
		}
	}
}

// WithWeight sets the weight.
func WithWeight(weight int) Option {
	return func(o *options) {
		if weight > 0 {
			o.weight = weight
		} else {
			log.Warnf("the specified weight is less than zero and will be ignored")
		}
	}
}

// WithMetadata sets the metadata.
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

// WithLinkerConnNum sets the number of connections.
func WithLinkerConnNum(connNum int) Option {
	return func(o *options) {
		if connNum > 0 {
			o.linker.connNum = connNum
		} else {
			log.Warnf("the specified linker's connNum is less than zero and will be ignored")
		}
	}
}

// WithLinkerCallTimeout sets the RPC call timeout.
func WithLinkerCallTimeout(callTimeout time.Duration) Option {
	return func(o *options) {
		if callTimeout >= 0 {
			o.linker.callTimeout = callTimeout
		} else {
			log.Warnf("the specified linker's callTimeout is less than zero and will be ignored")
		}
	}
}

// WithLinkerDialTimeout sets the internal RPC dial timeout.
func WithLinkerDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) {
		if dialTimeout >= 0 {
			o.linker.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified linker's dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithLinkerDialRetryTimes sets the number of internal RPC dial retries.
func WithLinkerDialRetryTimes(dialRetryTimes int) Option {
	return func(o *options) {
		if dialRetryTimes >= 0 {
			o.linker.dialRetryTimes = dialRetryTimes
		} else {
			log.Warnf("the specified linker's dialRetryTimes is less than zero and will be ignored")
		}
	}
}

// WithLinkerFaultRecoveryTime sets the internal RPC fault recovery time.
func WithLinkerFaultRecoveryTime(faultRecoveryTime time.Duration) Option {
	return func(o *options) {
		if faultRecoveryTime >= 0 {
			o.linker.faultRecoveryTime = faultRecoveryTime
		} else {
			log.Warnf("the specified linker's faultRecoveryTime is less than zero and will be ignored")
		}
	}
}

// WithLinkerCommandQueueSize sets the message queue size.
func WithLinkerCommandQueueSize(commandQueueSize int32) Option {
	return func(o *options) {
		if commandQueueSize > 0 {
			o.linker.commandQueueSize = commandQueueSize
		} else {
			log.Warnf("the specified linker's messageQueueSize is less than zero and will be ignored")
		}
	}
}

// WithLinkerCommandWriteTimeout sets the write timeout.
func WithLinkerCommandWriteTimeout(commandWriteTimeout time.Duration) Option {
	return func(o *options) {
		if commandWriteTimeout >= 0 {
			o.linker.commandWriteTimeout = commandWriteTimeout
		} else {
			log.Warnf("the specified linker's commandWriteTimeout is less than zero and will be ignored")
		}
	}
}
