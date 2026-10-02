package kcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientDialAddr          = "127.0.0.1:3553"
	defaultClientDialTimeout       = "3s"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
	defaultClientCloseTimeout      = "0s"
	defaultClientMtu               = 1400
	defaultClientWriteDelay        = true
)

var (
	defaultClientNoDelay    = []int{1, 10, 2, 1}
	defaultClientWindowSize = []int{32, 32}
)

const (
	defaultClientDialAddrKey          = "etc.network.kcp.client.addr"
	defaultClientDialTimeoutKey       = "etc.network.kcp.client.dialTimeout"
	defaultClientDialTimeoutLegacyKey = "etc.network.kcp.client.timeout"
	defaultClientHeartbeatIntervalKey = "etc.network.kcp.client.heartbeatInterval"
	defaultClientWriteTimeoutKey      = "etc.network.kcp.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.kcp.client.writeQueueSize"
	defaultClientCloseTimeoutKey      = "etc.network.kcp.client.closeTimeout"
	defaultClientMtuKey               = "etc.network.kcp.client.mtu"
	defaultClientNoDelayKey           = "etc.network.kcp.client.noDelay"
	defaultClientAckNoDelayKey        = "etc.network.kcp.client.ackNoDelay"
	defaultClientWriteDelayKey        = "etc.network.kcp.client.writeDelay"
	defaultClientWindowSizeKey        = "etc.network.kcp.client.windowSize"
	defaultClientReadBufferKey        = "etc.network.kcp.client.readBuffer"
	defaultClientWriteBufferKey       = "etc.network.kcp.client.writeBuffer"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // Address
	dialTimeout       time.Duration // Dial timeout, 3s by default
	writeTimeout      time.Duration // Write timeout, no timeout by default
	writeQueueSize    int           // Write queue size, 1024 by default
	heartbeatInterval time.Duration // Heartbeat interval, 10s by default
	closeTimeout      time.Duration // Graceful close timeout, 0s by default, meaning unlimited
	mtu               int           // Maximum transmission unit, unset by default
	noDelay           []int         // Whether to enable no-delay mode, unset by default
	ackNoDelay        bool          // Whether to enable ACK no-delay, unset by default
	writeDelay        bool          // Whether to enable write delay, unset by default
	windowSize        []int         // Window size, unset by default
	readBuffer        int           // Read buffer size, unset by default
	writeBuffer       int           // Write buffer size, unset by default
}

// defaultClientOptions returns the default client options.
//
// Every option is read from the configuration center, falling back to the built-in defaults.
func defaultClientOptions() *clientOptions {
	opts := &clientOptions{}

	if addr := etc.Get(defaultClientDialAddrKey, defaultClientDialAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultClientDialAddr
	}

	// Prefer the new dialTimeout key that aligns with TCP naming, falling back to the legacy
	// timeout key for compatibility when it is absent.
	if dialTimeout := etc.Get(defaultClientDialTimeoutKey, etc.Get(defaultClientDialTimeoutLegacyKey, defaultClientDialTimeout)).Duration(); dialTimeout > 0 {
		opts.dialTimeout = dialTimeout
	} else {
		opts.dialTimeout = xconv.Duration(defaultClientDialTimeout)
	}

	if writeTimeout := etc.Get(defaultClientWriteTimeoutKey, defaultClientWriteTimeout).Duration(); writeTimeout >= 0 {
		opts.writeTimeout = writeTimeout
	} else {
		opts.writeTimeout = xconv.Duration(defaultClientWriteTimeout)
	}

	if writeQueueSize := etc.Get(defaultClientWriteQueueSizeKey, defaultClientWriteQueueSize).Int(); writeQueueSize > 0 {
		opts.writeQueueSize = writeQueueSize
	} else {
		opts.writeQueueSize = defaultClientWriteQueueSize
	}

	if heartbeatInterval := etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(); heartbeatInterval >= 0 {
		opts.heartbeatInterval = heartbeatInterval
	} else {
		opts.heartbeatInterval = xconv.Duration(defaultClientHeartbeatInterval)
	}

	if closeTimeout := etc.Get(defaultClientCloseTimeoutKey, defaultClientCloseTimeout).Duration(); closeTimeout >= 0 {
		opts.closeTimeout = closeTimeout
	} else {
		opts.closeTimeout = xconv.Duration(defaultClientCloseTimeout)
	}

	opts.mtu = etc.Get(defaultClientMtuKey, defaultClientMtu).Int()
	opts.noDelay = etc.Get(defaultClientNoDelayKey, defaultClientNoDelay).Ints()
	opts.ackNoDelay = etc.Get(defaultClientAckNoDelayKey).Bool()
	opts.writeDelay = etc.Get(defaultClientWriteDelayKey, defaultClientWriteDelay).Bool()
	opts.windowSize = etc.Get(defaultClientWindowSizeKey, defaultClientWindowSize).Ints()
	opts.readBuffer = int(etc.Get(defaultClientReadBufferKey).B())
	opts.writeBuffer = int(etc.Get(defaultClientWriteBufferKey).B())

	return opts
}

// WithClientDialAddr sets the dial address.
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithClientDialTimeout sets the dial timeout. A negative dialTimeout is ignored.
func WithClientDialTimeout(dialTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if dialTimeout >= 0 {
			o.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientHeartbeatInterval sets the heartbeat interval.
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithClientMtu sets the maximum transmission unit.
func WithClientMtu(mtu int) ClientOption {
	return func(o *clientOptions) { o.mtu = mtu }
}

// WithClientNoDelay sets whether to enable no-delay mode.
//
// noDelay must be a 4-tuple (nodelay, interval, resend, nc).
func WithClientNoDelay(noDelay []int) ClientOption {
	return func(o *clientOptions) {
		if len(noDelay) == 4 {
			o.noDelay = noDelay
		} else {
			log.Warnf("the specified noDelay must be a 4-tuple and will be ignored")
		}
	}
}

// WithClientAckNoDelay sets whether to enable ACK no-delay.
func WithClientAckNoDelay(ackNoDelay bool) ClientOption {
	return func(o *clientOptions) { o.ackNoDelay = ackNoDelay }
}

// WithClientWriteDelay sets whether to enable write delay.
func WithClientWriteDelay(writeDelay bool) ClientOption {
	return func(o *clientOptions) { o.writeDelay = writeDelay }
}

// WithClientWindowSize sets the window size.
//
// windowSize must be a 2-tuple (sndwnd, rcvwnd).
func WithClientWindowSize(windowSize []int) ClientOption {
	return func(o *clientOptions) {
		if len(windowSize) == 2 {
			o.windowSize = windowSize
		} else {
			log.Warnf("the specified windowSize must be a 2-tuple and will be ignored")
		}
	}
}

// WithClientReadBuffer sets the read buffer size.
func WithClientReadBuffer(readBuffer int) ClientOption {
	return func(o *clientOptions) { o.readBuffer = readBuffer }
}

// WithClientWriteBuffer sets the write buffer size.
func WithClientWriteBuffer(writeBuffer int) ClientOption {
	return func(o *clientOptions) { o.writeBuffer = writeBuffer }
}

// WithClientWriteTimeout sets the write timeout. A negative writeTimeout is ignored.
func WithClientWriteTimeout(writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientWriteQueueSize sets the write queue size. A value less than or equal to 0 is ignored.
func WithClientWriteQueueSize(writeQueueSize int) ClientOption {
	return func(o *clientOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithClientCloseTimeout sets the graceful close timeout.
//
// When the write queue has not drained before the timeout elapses, the connection gives up waiting
// and is closed forcibly. A value of 0, the default, means unlimited.
func WithClientCloseTimeout(closeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if closeTimeout >= 0 {
			o.closeTimeout = closeTimeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}
