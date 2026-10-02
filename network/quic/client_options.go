package quic

import (
	"crypto/tls"
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientAddr              = "127.0.0.1:3553"
	defaultClientDialTimeout       = "5s"
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientAddrKey              = "etc.network.quic.client.addr"
	defaultClientCAFileKey            = "etc.network.quic.client.caFile"
	defaultClientServerNameKey        = "etc.network.quic.client.serverName"
	defaultClientDialTimeoutKey       = "etc.network.quic.client.dialTimeout"
	defaultClientWriteTimeoutKey      = "etc.network.quic.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.quic.client.writeQueueSize"
	defaultClientHeartbeatIntervalKey = "etc.network.quic.client.heartbeatInterval"
	defaultClientCloseTimeoutKey      = "etc.network.quic.client.closeTimeout"
)

// ClientOption is a functional option for configuring a client.
type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // Address
	tlsErr            error         // TLS configuration error
	closeTimeout      time.Duration // Graceful close timeout
	tlsConfig         *tls.Config   // TLS config
	dialTimeout       time.Duration // Dial timeout, 5s by default
	writeTimeout      time.Duration // Write timeout, no timeout by default
	writeQueueSize    int           // Write queue size, 1024 by default
	heartbeatInterval time.Duration // Heartbeat interval, 10s by default
}

// defaultClientOptions builds the default client options.
//
// It reads the environment configuration (etc.network.quic.client.*) first and falls back to the
// built-in defaults when an entry is missing, and it tries to load the CA certificate to build the
// TLS configuration.
func defaultClientOptions() *clientOptions {
	opts := &clientOptions{closeTimeout: defaultCloseTimeout}

	if addr := etc.Get(defaultClientAddrKey, defaultClientAddr).String(); addr != "" {
		opts.addr = addr
	} else {
		opts.addr = defaultClientAddr
	}

	if dialTimeout := etc.Get(defaultClientDialTimeoutKey, defaultClientDialTimeout).Duration(); dialTimeout >= 0 {
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

	caFile := etc.Get(defaultClientCAFileKey).String()
	if timeout := etc.Get(defaultClientCloseTimeoutKey, defaultCloseTimeout).Duration(); timeout > 0 {
		opts.closeTimeout = timeout
	}
	serverName := etc.Get(defaultClientServerNameKey).String()

	if caFile != "" || serverName != "" {
		if config, err := makeClientTLSConfig(caFile, serverName); err != nil {
			opts.tlsErr = err
		} else {
			opts.tlsConfig = config
			opts.tlsErr = nil
		}
	}

	return opts
}

// WithClientAddr sets the dial address. An empty addr is ignored.
func WithClientAddr(addr string) ClientOption {
	return func(o *clientOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithClientCredentials sets the CA certificate file and the name verified against the server
// certificate.
func WithClientCredentials(caFile string, serverName string) ClientOption {
	return func(o *clientOptions) {
		if caFile != "" || serverName != "" {
			if config, err := makeClientTLSConfig(caFile, serverName); err != nil {
				o.tlsErr = err
			} else {
				o.tlsConfig = config
				o.tlsErr = nil
			}
		} else {
			log.Warnf("the specified caFile or serverName is empty and will be ignored")
		}
	}
}

// WithClientTLSConfig sets the TLS configuration.
func WithClientTLSConfig(tlsConfig *tls.Config) ClientOption {
	return func(o *clientOptions) {
		o.tlsConfig = tlsConfig
		o.tlsErr = nil
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

// WithClientHeartbeatInterval sets the heartbeat interval. A negative heartbeatInterval is ignored.
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithClientCloseTimeout sets the graceful close timeout.
//
// The timeout is used both as the drain wait limit for a graceful close and as the retransmission
// dwell time after closing. A value less than or equal to 0 is ignored.
func WithClientCloseTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if timeout > 0 {
			o.closeTimeout = timeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}
