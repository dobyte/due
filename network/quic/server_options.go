package quic

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
	defaultServerHandshakeTimeout   = "5s"
)

const (
	defaultServerAddrKey               = "etc.network.quic.server.addr"
	defaultServerCertFileKey           = "etc.network.quic.server.certFile"
	defaultServerKeyFileKey            = "etc.network.quic.server.keyFile"
	defaultServerMaxConnNumKey         = "etc.network.quic.server.maxConnNum"
	defaultServerWriteTimeoutKey       = "etc.network.quic.server.writeTimeout"
	defaultServerWriteQueueSizeKey     = "etc.network.quic.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey  = "etc.network.quic.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey = "etc.network.quic.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey   = "etc.network.quic.server.authorizeTimeout"
	defaultServerHandshakeTimeoutKey   = "etc.network.quic.server.handshakeTimeout"
	defaultServerCloseTimeoutKey       = "etc.network.quic.server.closeTimeout"
)

const (
	// RespHeartbeat is responsive heartbeat: a heartbeat is sent back only when a heartbeat from
	// the peer is received.
	RespHeartbeat HeartbeatMechanism = "resp"
	// TickHeartbeat is active periodic heartbeat: heartbeat packets are dispatched at the
	// heartbeat interval.
	TickHeartbeat HeartbeatMechanism = "tick"
)

// HeartbeatMechanism is a heartbeat mechanism.
type HeartbeatMechanism string

// ServerOption is a functional option for configuring a server.
type ServerOption func(o *serverOptions)

type serverOptions struct {
	closeTimeout       time.Duration      // Graceful close timeout
	addr               string             // Listen address, 0.0.0.0:3553 by default
	certFile           string             // Certificate file
	keyFile            string             // Private key file
	maxConnNum         int                // Maximum number of connections, 5000 by default
	writeTimeout       time.Duration      // Write timeout, no timeout by default
	writeQueueSize     int                // Write queue size, 1024 by default
	heartbeatInterval  time.Duration      // Heartbeat detection interval, 10s by default
	heartbeatMechanism HeartbeatMechanism // Heartbeat mechanism, resp by default
	authorizeTimeout   time.Duration      // Authorize timeout, 0s by default, meaning no check
	handshakeTimeout   time.Duration      // Handshake timeout, 5s by default
}

// defaultServerOptions builds the default server options.
//
// It reads the environment configuration (etc.network.quic.server.*) first and falls back to the
// built-in defaults when an entry is missing.
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{closeTimeout: defaultCloseTimeout}
	if timeout := etc.Get(defaultServerCloseTimeoutKey, defaultCloseTimeout).Duration(); timeout > 0 {
		opts.closeTimeout = timeout
	}
	opts.certFile = etc.Get(defaultServerCertFileKey).String()
	opts.keyFile = etc.Get(defaultServerKeyFileKey).String()

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

	if handshakeTimeout := etc.Get(defaultServerHandshakeTimeoutKey, defaultServerHandshakeTimeout).Duration(); handshakeTimeout > 0 {
		opts.handshakeTimeout = handshakeTimeout
	} else {
		opts.handshakeTimeout = xconv.Duration(defaultServerHandshakeTimeout)
	}

	return opts
}

// WithServerAddr sets the listen address. An empty addr is ignored.
func WithServerAddr(addr string) ServerOption {
	return func(o *serverOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithServerCredentials sets the server certificate and private key.
func WithServerCredentials(certFile, keyFile string) ServerOption {
	return func(o *serverOptions) {
		if certFile != "" && keyFile != "" {
			o.certFile, o.keyFile = certFile, keyFile
		} else {
			log.Warnf("the specified certFile or keyFile is empty and will be ignored")
		}
	}
}

// WithServerMaxConnNum sets the maximum number of connections. A value less than or equal to 0 is
// ignored.
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) {
		if maxConnNum > 0 {
			o.maxConnNum = maxConnNum
		} else {
			log.Warnf("the specified maxConnNum is less than zero and will be ignored")
		}
	}
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

// WithServerHeartbeatInterval sets the heartbeat detection interval. A negative heartbeatInterval
// is ignored.
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatMechanism sets the heartbeat mechanism. The value must be [RespHeartbeat] or
// [TickHeartbeat].
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) {
		if heartbeatMechanism == RespHeartbeat || heartbeatMechanism == TickHeartbeat {
			o.heartbeatMechanism = heartbeatMechanism
		} else {
			log.Warnf("the specified heartbeatMechanism is %v and will be ignored", heartbeatMechanism)
		}
	}
}

// WithServerAuthorizeTimeout sets the authorize timeout. A negative authorizeTimeout is ignored and
// 0 means no check.
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if authorizeTimeout >= 0 {
			o.authorizeTimeout = authorizeTimeout
		} else {
			log.Warnf("the specified authorizeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerHandshakeTimeout sets the handshake timeout. A value less than or equal to 0 is ignored.
func WithServerHandshakeTimeout(handshakeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if handshakeTimeout > 0 {
			o.handshakeTimeout = handshakeTimeout
		} else {
			log.Warnf("the specified handshakeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerCloseTimeout sets the graceful close timeout.
//
// The timeout is used both as the drain wait limit for a graceful close and as the retransmission
// dwell time after closing. A value less than or equal to 0 is ignored.
func WithServerCloseTimeout(timeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if timeout > 0 {
			o.closeTimeout = timeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}
