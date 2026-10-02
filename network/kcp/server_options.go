// Package kcp implements the KCP-based server and client of the due network layer.
package kcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultServerAddr               = ":3553"
	defaultServerMaxConnNum         = 5000
	defaultServerWriteTimeout       = "0s"
	defaultServerWriteQueueSize     = 1024
	defaultServerHeartbeatInterval  = "10s"
	defaultServerHeartbeatMechanism = "resp"
	defaultServerAuthorizeTimeout   = "0s"
	defaultServerCloseTimeout       = "0s"
	defaultServerMtu                = 1400
	defaultServerWriteDelay         = true
)

var (
	defaultServerNoDelay    = []int{1, 10, 2, 1}
	defaultServerWindowSize = []int{32, 32}
)

const (
	defaultServerAddrKey                = "etc.network.kcp.server.addr"
	defaultServerMaxConnNumKey          = "etc.network.kcp.server.maxConnNum"
	defaultServerWriteTimeoutKey        = "etc.network.kcp.server.writeTimeout"
	defaultServerWriteQueueSizeKey      = "etc.network.kcp.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey   = "etc.network.kcp.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey  = "etc.network.kcp.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey    = "etc.network.kcp.server.authorizeTimeout"
	defaultServerCloseTimeoutKey        = "etc.network.kcp.server.closeTimeout"
	defaultServerEnableProxyProtocolKey = "etc.network.kcp.server.enableProxyProtocol"
	defaultServerMtuKey                 = "etc.network.kcp.server.mtu"
	defaultServerNoDelayKey             = "etc.network.kcp.server.noDelay"
	defaultServerAckNoDelayKey          = "etc.network.kcp.server.ackNoDelay"
	defaultServerWriteDelayKey          = "etc.network.kcp.server.writeDelay"
	defaultServerWindowSizeKey          = "etc.network.kcp.server.windowSize"
	defaultServerReadBufferKey          = "etc.network.kcp.server.readBuffer"
	defaultServerWriteBufferKey         = "etc.network.kcp.server.writeBuffer"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // Responsive heartbeat
	TickHeartbeat HeartbeatMechanism = "tick" // Active periodic heartbeat
)

type HeartbeatMechanism string

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr                string             // Listen address
	maxConnNum          int                // Maximum number of connections
	writeTimeout        time.Duration      // Write timeout, no timeout by default
	writeQueueSize      int                // Write queue size, 1024 by default
	heartbeatInterval   time.Duration      // Heartbeat detection interval, 10s by default
	heartbeatMechanism  HeartbeatMechanism // Heartbeat mechanism, resp by default
	authorizeTimeout    time.Duration      // Authorize timeout, 0s by default, meaning no check
	closeTimeout        time.Duration      // Graceful close timeout, 0s by default, meaning unlimited
	mtu                 int                // Maximum transmission unit, unset by default
	noDelay             []int              // Whether to enable no-delay mode, unset by default
	ackNoDelay          bool               // Whether to enable ACK no-delay, unset by default
	writeDelay          bool               // Whether to enable write delay, unset by default
	windowSize          []int              // Window size, unset by default
	readBuffer          int                // Read buffer size, unset by default
	writeBuffer         int                // Write buffer size, unset by default
	enableProxyProtocol bool               // Whether to enable PROXY protocol, false by default
}

// defaultServerOptions returns the default server options.
//
// Every option is read from the configuration center, falling back to the built-in defaults.
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{}
	opts.enableProxyProtocol = etc.Get(defaultServerEnableProxyProtocolKey).Bool()

	if addr := etc.Get(defaultServerAddrKey, defaultServerAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultServerAddr
	}

	if maxConnNum := etc.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(); maxConnNum > 0 {
		opts.maxConnNum = maxConnNum
	} else {
		opts.maxConnNum = defaultServerMaxConnNum
	}

	if writeTimeout := etc.Get(defaultServerWriteTimeoutKey, defaultServerWriteTimeout).Duration(); writeTimeout >= 0 {
		opts.writeTimeout = writeTimeout
	} else {
		opts.writeTimeout = xconv.Duration(defaultServerWriteTimeout)
	}

	if writeQueueSize := etc.Get(defaultServerWriteQueueSizeKey, defaultServerWriteQueueSize).Int(); writeQueueSize > 0 {
		opts.writeQueueSize = writeQueueSize
	} else {
		opts.writeQueueSize = defaultServerWriteQueueSize
	}

	if heartbeatInterval := etc.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration(); heartbeatInterval >= 0 {
		opts.heartbeatInterval = heartbeatInterval
	} else {
		opts.heartbeatInterval = xconv.Duration(defaultServerHeartbeatInterval)
	}

	switch heartbeatMechanism := HeartbeatMechanism(etc.Get(defaultServerHeartbeatMechanismKey, defaultServerHeartbeatMechanism).String()); heartbeatMechanism {
	case RespHeartbeat, TickHeartbeat:
		opts.heartbeatMechanism = heartbeatMechanism
	default:
		opts.heartbeatMechanism = defaultServerHeartbeatMechanism
	}

	if authorizeTimeout := etc.Get(defaultServerAuthorizeTimeoutKey, defaultServerAuthorizeTimeout).Duration(); authorizeTimeout >= 0 {
		opts.authorizeTimeout = authorizeTimeout
	} else {
		opts.authorizeTimeout = xconv.Duration(defaultServerAuthorizeTimeout)
	}

	if closeTimeout := etc.Get(defaultServerCloseTimeoutKey, defaultServerCloseTimeout).Duration(); closeTimeout >= 0 {
		opts.closeTimeout = closeTimeout
	} else {
		opts.closeTimeout = xconv.Duration(defaultServerCloseTimeout)
	}

	opts.mtu = etc.Get(defaultServerMtuKey, defaultServerMtu).Int()
	opts.noDelay = etc.Get(defaultServerNoDelayKey, defaultServerNoDelay).Ints()
	opts.ackNoDelay = etc.Get(defaultServerAckNoDelayKey).Bool()
	opts.writeDelay = etc.Get(defaultServerWriteDelayKey, defaultServerWriteDelay).Bool()
	opts.windowSize = etc.Get(defaultServerWindowSizeKey, defaultServerWindowSize).Ints()
	opts.readBuffer = int(etc.Get(defaultServerReadBufferKey).B())
	opts.writeBuffer = int(etc.Get(defaultServerWriteBufferKey).B())

	return opts
}

// WithServerListenAddr sets the listen address.
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithServerMaxConnNum sets the maximum number of connections.
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) {
		if maxConnNum > 0 {
			o.maxConnNum = maxConnNum
		} else {
			log.Warnf("the specified maxConnNum is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatInterval sets the heartbeat detection interval.
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatMechanism sets the heartbeat mechanism.
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) {
		if heartbeatMechanism == RespHeartbeat || heartbeatMechanism == TickHeartbeat {
			o.heartbeatMechanism = heartbeatMechanism
		} else {
			log.Warnf("the specified heartbeatMechanism is %v and will be ignored", heartbeatMechanism)
		}
	}
}

// WithServerAuthorizeTimeout sets the authorize timeout.
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if authorizeTimeout >= 0 {
			o.authorizeTimeout = authorizeTimeout
		} else {
			log.Warnf("the specified authorizeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerCloseTimeout sets the graceful close timeout.
//
// When the write queue has not drained before the timeout elapses, the connection gives up waiting
// and is closed forcibly. A value of 0, the default, means unlimited.
func WithServerCloseTimeout(closeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if closeTimeout >= 0 {
			o.closeTimeout = closeTimeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerMtu sets the maximum transmission unit.
func WithServerMtu(mtu int) ServerOption {
	return func(o *serverOptions) { o.mtu = mtu }
}

// WithServerNoDelay sets whether to enable no-delay mode.
func WithServerNoDelay(noDelay []int) ServerOption {
	return func(o *serverOptions) { o.noDelay = noDelay }
}

// WithServerAckNoDelay sets whether to enable ACK no-delay.
func WithServerAckNoDelay(ackNoDelay bool) ServerOption {
	return func(o *serverOptions) { o.ackNoDelay = ackNoDelay }
}

// WithServerWriteDelay sets whether to enable write delay.
func WithServerWriteDelay(writeDelay bool) ServerOption {
	return func(o *serverOptions) { o.writeDelay = writeDelay }
}

// WithServerWindowSize sets the window size.
func WithServerWindowSize(windowSize []int) ServerOption {
	return func(o *serverOptions) { o.windowSize = windowSize }
}

// WithServerReadBuffer sets the read buffer size.
func WithServerReadBuffer(readBuffer int) ServerOption {
	return func(o *serverOptions) { o.readBuffer = readBuffer }
}

// WithServerWriteBuffer sets the write buffer size.
func WithServerWriteBuffer(writeBuffer int) ServerOption {
	return func(o *serverOptions) { o.writeBuffer = writeBuffer }
}

// WithServerWriteTimeout sets the write timeout. A negative writeTimeout is ignored.
func WithServerWriteTimeout(writeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerWriteQueueSize sets the write queue size. A value less than or equal to 0 is ignored.
func WithServerWriteQueueSize(writeQueueSize int) ServerOption {
	return func(o *serverOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}
