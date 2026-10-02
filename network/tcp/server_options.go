package tcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultServerAddr               = ":3553"
	defaultServerMaxConnNum         = 5000
	defaultServerReadBufferSize     = 4096
	defaultServerWriteTimeout       = "0s"
	defaultServerWriteQueueSize     = 1024
	defaultServerHeartbeatInterval  = "10s"
	defaultServerHeartbeatMechanism = "resp"
	defaultServerAuthorizeTimeout   = "0s"
	defaultServerCloseTimeout       = "0s"
)

const (
	defaultServerAddrKey                = "etc.network.tcp.server.addr"
	defaultServerCertFileKey            = "etc.network.tcp.server.certFile"
	defaultServerKeyFileKey             = "etc.network.tcp.server.keyFile"
	defaultServerMaxConnNumKey          = "etc.network.tcp.server.maxConnNum"
	defaultServerReadBufferSizeKey      = "etc.network.tcp.server.readBufferSize"
	defaultServerWriteTimeoutKey        = "etc.network.tcp.server.writeTimeout"
	defaultServerWriteQueueSizeKey      = "etc.network.tcp.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey   = "etc.network.tcp.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey  = "etc.network.tcp.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey    = "etc.network.tcp.server.authorizeTimeout"
	defaultServerCloseTimeoutKey        = "etc.network.tcp.server.closeTimeout"
	defaultServerEnableProxyProtocolKey = "etc.network.tcp.server.enableProxyProtocol"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // Responsive heartbeat
	TickHeartbeat HeartbeatMechanism = "tick" // Active tick heartbeat
)

type HeartbeatMechanism string

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr                string             // Listen address, defaults to 0.0.0.0:3553
	certFile            string             // Certificate file
	keyFile             string             // Key file
	maxConnNum          int                // Maximum number of connections, defaults to 5000
	readBufferSize      int                // Read buffer size, defaults to 4096
	writeTimeout        time.Duration      // Write timeout, defaults to no timeout
	writeQueueSize      int                // Write queue size, defaults to 1024
	heartbeatInterval   time.Duration      // Heartbeat check interval, defaults to 10s
	heartbeatMechanism  HeartbeatMechanism // Heartbeat mechanism, defaults to resp
	authorizeTimeout    time.Duration      // Authorization timeout, defaults to 0s (no check)
	closeTimeout        time.Duration      // Graceful close timeout, defaults to 0s (no limit)
	enableProxyProtocol bool               // Whether to enable ProxyProtocol, defaults to false
}

// defaultServerOptions builds the default server options.
//
// It reads the environment config (etc.network.tcp.server.*) first and falls back to the built-in
// defaults when a value is missing.
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{}
	opts.certFile = etc.Get(defaultServerCertFileKey).String()
	opts.keyFile = etc.Get(defaultServerKeyFileKey).String()
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

	if readBufferSize := etc.Get(defaultServerReadBufferSizeKey, defaultServerReadBufferSize).Int(); readBufferSize > 0 {
		opts.readBufferSize = readBufferSize
	} else {
		opts.readBufferSize = defaultServerReadBufferSize
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

	return opts
}

// WithServerAddr sets the listen address.
func WithServerAddr(addr string) ServerOption {
	return func(o *serverOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithServerCredentials sets the server certificate and key.
func WithServerCredentials(certFile, keyFile string) ServerOption {
	return func(o *serverOptions) {
		if certFile != "" && keyFile != "" {
			o.certFile, o.keyFile = certFile, keyFile
		} else {
			log.Warnf("the specified certFile or keyFile is empty and will be ignored")
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

// WithServerReadBufferSize sets the read buffer size.
func WithServerReadBufferSize(readBufferSize int) ServerOption {
	return func(o *serverOptions) {
		if readBufferSize > 0 {
			o.readBufferSize = readBufferSize
		} else {
			log.Warnf("the specified readBufferSize is less than zero and will be ignored")
		}
	}
}

// WithServerWriteTimeout sets the write timeout.
func WithServerWriteTimeout(writeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerWriteQueueSize sets the write queue size.
func WithServerWriteQueueSize(writeQueueSize int) ServerOption {
	return func(o *serverOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatInterval sets the heartbeat check interval.
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

// WithServerAuthorizeTimeout sets the authorization timeout.
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
// When the write queue has not drained before the timeout elapses, the wait is abandoned and the
// connection is closed forcibly. The default value of 0 means no limit.
func WithServerCloseTimeout(closeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if closeTimeout >= 0 {
			o.closeTimeout = closeTimeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerEnableProxyProtocol sets whether to enable ProxyProtocol.
func WithServerEnableProxyProtocol(enableProxyProtocol bool) ServerOption {
	return func(o *serverOptions) { o.enableProxyProtocol = enableProxyProtocol }
}
