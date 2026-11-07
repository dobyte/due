package gate

import (
	"context"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/aead/chacha20poly1305"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// Default option values.
const (
	defaultName                      = "gate"          // default name
	defaultDispatch                  = cluster.Random  // default dispatch strategy of stateless route messages
	defaultAddr                      = ":0"            // connector listen address
	defaultLinkerConnNum             = 5               // default number of connections
	defaultLinkerCallTimeout         = "3s"            // default call timeout
	defaultLinkerDialTimeout         = "3s"            // default dial timeout
	defaultLinkerDialRetryTimes      = 3               // default number of dial retries
	defaultLinkerFaultRecoveryTime   = "5s"            // default fault recovery time
	defaultLinkerCommandQueueSize    = 4096            // default message queue size
	defaultLinkerCommandWriteTimeout = "0s"            // default write timeout
	defaultSecurityHandshakeTimeout  = 5 * time.Second // default session security handshake timeout
	defaultLimiterPeriod             = "1s"            // default connection rate limit period
	defaultLimiterCloseThreshold     = 100             // default consecutive limited messages before closing
	defaultGuardPeriod               = "1s"            // default IP admission rate period
	defaultGuardStrikeThreshold      = 0               // automatic IP bans are opt-in
	defaultGuardStrikeWindow         = "5m"            // default window of the violations that lead to a ban
	defaultGuardMinBanDuration       = "1m"            // default first ban duration
	defaultGuardMaxBanDuration       = "1h"            // default maximum ban duration
)

// Configuration keys for the gate options.
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
	defaultLimiterQuotaKey              = "etc.cluster.gate.limiter.quota"
	defaultLimiterPeriodKey             = "etc.cluster.gate.limiter.period"
	defaultLimiterBurstKey              = "etc.cluster.gate.limiter.burst"
	defaultLimiterCloseThresholdKey     = "etc.cluster.gate.limiter.closeThreshold"
	defaultGuardMaxConnsKey             = "etc.cluster.gate.guard.maxConns"
	defaultGuardMaxEntriesKey           = "etc.cluster.gate.guard.maxEntries"
	defaultGuardQuotaKey                = "etc.cluster.gate.guard.quota"
	defaultGuardPeriodKey               = "etc.cluster.gate.guard.period"
	defaultGuardBurstKey                = "etc.cluster.gate.guard.burst"
	defaultGuardWhitelistKey            = "etc.cluster.gate.guard.whitelist"
	defaultGuardStrikeThresholdKey      = "etc.cluster.gate.guard.strikeThreshold"
	defaultGuardStrikeWindowKey         = "etc.cluster.gate.guard.strikeWindow"
	defaultGuardMinBanDurationKey       = "etc.cluster.gate.guard.minBanDuration"
	defaultGuardMaxBanDurationKey       = "etc.cluster.gate.guard.maxBanDuration"
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
	guard    *guardOptions     // per-IP admission and ban options
	limiter  *limiterOptions   // connection rate limiting options
	security *securityOptions  // session security options
}

// defaultOptions returns the default options.
//
// It reads each default value from the configuration center and builds the default options.
func defaultOptions() *options {
	opts := &options{}
	opts.ctx = context.Background()
	opts.expose = etc.Get(defaultExposeKey).Bool()
	opts.linker = &linkerOptions{}
	opts.limiter = &limiterOptions{closeThreshold: defaultLimiterCloseThreshold}
	opts.guard = &guardOptions{
		strikeThreshold: defaultGuardStrikeThreshold,
		strikeWindow:    xconv.Duration(defaultGuardStrikeWindow),
		minBanDuration:  xconv.Duration(defaultGuardMinBanDuration),
		maxBanDuration:  xconv.Duration(defaultGuardMaxBanDuration),
		maxEntries:      defaultGuardMaxEntries,
	}
	opts.metadata = make(map[string]string)
	opts.security = &securityOptions{timeout: defaultSecurityHandshakeTimeout}

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

	if quota := etc.Get(defaultLimiterQuotaKey).Float64(); quota > 0 {
		opts.limiter.quota = quota
	}

	if period := etc.Get(defaultLimiterPeriodKey, defaultLimiterPeriod).Duration(); period > 0 {
		opts.limiter.period = period
	} else {
		opts.limiter.period = xconv.Duration(defaultLimiterPeriod)
	}

	if burst := etc.Get(defaultLimiterBurstKey).Int(); burst > 0 {
		opts.limiter.burst = burst
	}

	if threshold := etc.Get(defaultLimiterCloseThresholdKey, defaultLimiterCloseThreshold).Int(); threshold >= 0 {
		opts.limiter.closeThreshold = threshold
	} else {
		opts.limiter.closeThreshold = defaultLimiterCloseThreshold
	}

	if maxConns := etc.Get(defaultGuardMaxConnsKey).Int(); maxConns > 0 {
		opts.guard.maxConns = maxConns
	}

	if maxEntries := etc.Get(defaultGuardMaxEntriesKey).Int(); maxEntries > 0 {
		opts.guard.maxEntries = maxEntries
	}

	if quota := etc.Get(defaultGuardQuotaKey).Float64(); quota > 0 {
		opts.guard.quota = quota
	}

	if period := etc.Get(defaultGuardPeriodKey, defaultGuardPeriod).Duration(); period > 0 {
		opts.guard.period = period
	} else {
		opts.guard.period = xconv.Duration(defaultGuardPeriod)
	}

	if burst := etc.Get(defaultGuardBurstKey).Int(); burst > 0 {
		opts.guard.burst = burst
	}

	if threshold := etc.Get(defaultGuardStrikeThresholdKey, defaultGuardStrikeThreshold).Int(); threshold >= 0 {
		opts.guard.strikeThreshold = threshold
	} else {
		opts.guard.strikeThreshold = defaultGuardStrikeThreshold
	}

	if window := etc.Get(defaultGuardStrikeWindowKey, defaultGuardStrikeWindow).Duration(); window > 0 {
		opts.guard.strikeWindow = window
	} else {
		opts.guard.strikeWindow = xconv.Duration(defaultGuardStrikeWindow)
	}

	if duration := etc.Get(defaultGuardMinBanDurationKey, defaultGuardMinBanDuration).Duration(); duration > 0 {
		opts.guard.minBanDuration = duration
	} else {
		opts.guard.minBanDuration = xconv.Duration(defaultGuardMinBanDuration)
	}

	if duration := etc.Get(defaultGuardMaxBanDurationKey, defaultGuardMaxBanDuration).Duration(); duration > 0 {
		opts.guard.maxBanDuration = duration
	} else {
		opts.guard.maxBanDuration = xconv.Duration(defaultGuardMaxBanDuration)
	}

	if whitelist := etc.Get(defaultGuardWhitelistKey).String(); whitelist != "" {
		for _, item := range strings.Split(whitelist, ",") {
			if item = strings.TrimSpace(item); item != "" {
				opts.guard.whitelist = append(opts.guard.whitelist, item)
			}
		}
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

// WithSessionSecurity enables the session security between the client and the gate.
//
// signer signs the ServerHello so that the client can verify the gate with the pinned public key.
// When no suite is given, the handshake defaults are used. When enabled, the payload of client
// messages is decrypted by the gate and re-encrypted for the downlink, so the node-side static
// encryptor must stay unconfigured.
func WithSessionSecurity(signer crypto.Signer, suites ...aead.Suite) Option {
	suites = slices.Clone(suites)

	return func(o *options) {
		if signer == nil {
			log.Warnf("the specified signer is nil and will be automatically ignored")
			return
		}

		o.security.signer = signer

		if len(suites) == 0 {
			o.security.suites = []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite}
		} else {
			o.security.suites = suites
		}
	}
}

// WithSessionSecurityHandshakeTimeout sets how long the gate waits for a client to complete the
// session security handshake. A zero duration disables the timeout.
func WithSessionSecurityHandshakeTimeout(timeout time.Duration) Option {
	return func(o *options) {
		if timeout >= 0 {
			o.security.timeout = timeout
		} else {
			log.Warnf("the specified session security handshake timeout is less than zero and will be ignored")
		}
	}
}

// WithLimiterQuota sets the rate limit quota. A zero quota disables rate limiting and overrides a
// quota configured through etc.
//
// The limit applies to business frames only. Heartbeat frames are handled before the receive
// handler. When session security is enabled, handshake frames are routed to the security manager
// and also bypass the limit.
func WithLimiterQuota(quota float64) Option {
	return func(o *options) {
		if quota < 0 || math.IsNaN(quota) || math.IsInf(quota, 0) {
			log.Warnf("the specified rate limit quota is invalid and will be ignored: %v", quota)
		} else {
			o.limiter.quota = quota
		}
	}
}

// WithLimiterPeriod sets the rate limit period.
func WithLimiterPeriod(period time.Duration) Option {
	return func(o *options) {
		if period <= 0 {
			log.Warnf("the specified rate limit period is non-positive and will be ignored: %v", period)
		} else {
			o.limiter.period = period
		}
	}
}

// WithLimiterBurst sets the rate limit burst.
func WithLimiterBurst(burst int) Option {
	return func(o *options) {
		if burst <= 0 {
			log.Warnf("the specified rate limit burst is non-positive and will be ignored: %v", burst)
		} else {
			o.limiter.burst = burst
		}
	}
}

// WithLimiterCloseThreshold sets the number of consecutive limited messages that closes the
// connection. A zero threshold disables closing.
func WithLimiterCloseThreshold(threshold int) Option {
	return func(o *options) {
		if threshold < 0 {
			log.Warnf("the specified rate limit close threshold is negative and will be ignored: %v",
				threshold)
		} else {
			o.limiter.closeThreshold = threshold
		}
	}
}

// WithGuardMaxConns sets the maximum number of concurrent connections allowed per IP. A zero value
// disables the limit.
func WithGuardMaxConns(maxConns int) Option {
	return func(o *options) {
		if maxConns < 0 {
			log.Warnf("the specified max connections is negative and will be ignored: %d", maxConns)
		} else {
			o.guard.maxConns = maxConns
		}
	}
}

// WithGuardMaxEntries sets the maximum number of IP entries retained by the guard. A non-positive
// value is ignored.
func WithGuardMaxEntries(maxEntries int) Option {
	return func(o *options) {
		if maxEntries <= 0 {
			log.Warnf("the specified max entries is non-positive and will be ignored: %d", maxEntries)
		} else {
			o.guard.maxEntries = maxEntries
		}
	}
}

// WithGuardAcceptRate sets the connection admission rate per IP. A zero quota disables the limit.
func WithGuardAcceptRate(quota float64, period time.Duration) Option {
	return func(o *options) {
		if quota < 0 || math.IsNaN(quota) || math.IsInf(quota, 0) {
			log.Warnf("the specified IP admission quota is invalid and will be ignored: %v", quota)
			return
		}

		if quota > 0 && period <= 0 {
			log.Warnf("the specified IP admission period is non-positive and will be ignored: %v", period)
			return
		}

		o.guard.quota = quota

		if period > 0 {
			o.guard.period = period
		}
	}
}

// WithGuardWhitelist sets the IP whitelist.
func WithGuardWhitelist(whitelist []string) Option {
	prefixes := slices.Clone(whitelist)

	return func(o *options) {
		if len(prefixes) == 0 {
			log.Warnf("the specified whitelist is empty and will be ignored")
		} else {
			o.guard.whitelist = prefixes
		}
	}
}

// WithGuardStrikeThreshold sets the number of violations within the strike window that triggers a
// ban. Violations include connection admission rejections and, when the connection limiter is
// enabled, close signals emitted by the message rate limiter. A zero threshold disables automatic
// bans.
func WithGuardStrikeThreshold(threshold int) Option {
	return func(o *options) {
		if threshold < 0 {
			log.Warnf("the specified strike threshold is negative and will be ignored: %d", threshold)
		} else {
			o.guard.strikeThreshold = threshold
		}
	}
}

// WithGuardStrikeWindow sets the strike window.
func WithGuardStrikeWindow(window time.Duration) Option {
	return func(o *options) {
		if window <= 0 {
			log.Warnf("the specified strike window is non-positive and will be ignored: %v", window)
		} else {
			o.guard.strikeWindow = window
		}
	}
}

// WithGuardMinBanDuration sets the first ban duration. A non-positive duration is ignored.
func WithGuardMinBanDuration(duration time.Duration) Option {
	return func(o *options) {
		if duration <= 0 {
			log.Warnf("the specified min ban duration is non-positive and will be ignored: %v", duration)
		} else {
			o.guard.minBanDuration = duration
		}
	}
}

// WithGuardMaxBanDuration sets the maximum ban duration. A non-positive duration is ignored.
func WithGuardMaxBanDuration(duration time.Duration) Option {
	return func(o *options) {
		if duration <= 0 {
			log.Warnf("the specified max ban duration is non-positive and will be ignored: %v", duration)
		} else {
			o.guard.maxBanDuration = duration
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
