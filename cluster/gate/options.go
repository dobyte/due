package gate

import (
	"context"
	"maps"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	defaultName                      = "gate"         // default name
	defaultDispatch                  = cluster.Random // default dispatch strategy of stateless route messages
	defaultAddr                      = ":0"           // connector listen address
	defaultLinkerConnNum             = 5              // default number of connections
	defaultLinkerCallTimeout         = "3s"           // default call timeout
	defaultLinkerDialTimeout         = "3s"           // default dial timeout
	defaultLinkerDialRetryTimes      = 3              // default number of dial retries
	defaultLinkerFaultRecoveryTime   = "5s"           // default fault recovery time
	defaultLinkerCommandQueueSize    = 4096           // default message queue size
	defaultLinkerCommandWriteTimeout = "0s"           // default write timeout
)

const (
	defaultIDKey                        = "etc.cluster.gate.id"
	defaultNameKey                      = "etc.cluster.gate.name"
	defaultDispatchKey                  = "etc.cluster.gate.dispatch"
	defaultMetadataKey                  = "etc.cluster.gate.metadata"
	defaultAddrKey                      = "etc.cluster.gate.addr"
	defaultExposeKey                    = "etc.cluster.gate.expose"
	defaultLinkerConnNumKey             = "etc.cluster.gate.linker.connNum"
	defaultLinkerCallTimeoutKey         = "etc.cluster.gate.linker.callTimeout"
	defaultLinkerDialTimeoutKey         = "etc.cluster.gate.linker.dialTimeout"
	defaultLinkerDialRetryTimesKey      = "etc.cluster.gate.linker.dialRetryTimes"
	defaultLinkerFaultRecoveryTimeKey   = "etc.cluster.gate.linker.faultRecoveryTime"
	defaultLinkerCommandQueueSizeKey    = "etc.cluster.gate.linker.commandQueueSize"
	defaultLinkerCommandWriteTimeoutKey = "etc.cluster.gate.linker.commandWriteTimeout"
)

// Option is a gate option.
type Option func(o *options)

// linkerOptions are the linker options.
type linkerOptions struct {
	connNum             int           // number of internal RPC dial connections
	callTimeout         time.Duration // internal RPC call timeout
	dialTimeout         time.Duration // internal RPC dial timeout
	dialRetryTimes      int           // number of internal RPC dial retries
	faultRecoveryTime   time.Duration // internal RPC fault recovery time
	commandQueueSize    int32         // message queue size
	commandWriteTimeout time.Duration // message write timeout
}

// options are the gate options.
type options struct {
	ctx      context.Context   // context
	id       string            // instance ID
	name     string            // instance name
	server   network.Server    // gate server
	locator  locate.Locator    // user locator
	registry registry.Registry // service registry
	dispatch cluster.Dispatch  // dispatch strategy of stateless route messages
	metadata map[string]string // metadata
	addr     string            // internal RPC listen address
	expose   bool              // whether the internal RPC address is exposed to the public network
	linker   *linkerOptions    // linker options
}

// defaultOptions returns the default options.
//
// It reads each default value from the configuration center and builds the default options.
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

	if addr := etc.Get(defaultAddrKey, defaultAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultAddr
	}

	if strategy := etc.Get(defaultDispatchKey).String(); strategy != "" {
		opts.dispatch = cluster.Dispatch(strategy)
	} else {
		opts.dispatch = defaultDispatch
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

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		} else {
			log.Warnf("the specified ctx is nil and will be ignored")
		}
	}
}

// WithServer sets the server.
func WithServer(server network.Server) Option {
	return func(o *options) {
		if server != nil {
			o.server = server
		} else {
			log.Warnf("the specified server is nil and will be ignored")
		}
	}
}

// WithLocator sets the user locator.
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

// WithDispatch sets the dispatch strategy of stateless route messages.
func WithDispatch(dispatch cluster.Dispatch) Option {
	return func(o *options) {
		if dispatch != "" {
			o.dispatch = dispatch
		} else {
			log.Warnf("the specified dispatch is empty and will be ignored")
		}
	}
}

// WithAddr sets the listen address.
func WithAddr(addr string) Option {
	return func(o *options) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithExpose sets whether the internal communication address is exposed to the public network.
func WithExpose(expose bool) Option {
	return func(o *options) { o.expose = expose }
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
