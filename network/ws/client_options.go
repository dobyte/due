package ws

import (
	"crypto/tls"
	"time"

	ctls "github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultClientUrl               = "ws://127.0.0.1:3553"
	defaultClientDialTimeout       = "3s"
	defaultClientReadBufferSize    = 4096
	defaultClientWriteTimeout      = "0s"
	defaultClientWriteQueueSize    = 1024
	defaultClientHeartbeatInterval = "10s"
	defaultClientCloseTimeout      = "0s"
	defaultClientEnableCompression = false
	defaultClientCompressionLevel  = 1
)

const (
	defaultClientUrlKey               = "etc.network.ws.client.url"
	defaultClientCAFileKey            = "etc.network.ws.client.caFile"
	defaultClientServerNameKey        = "etc.network.ws.client.serverName"
	defaultClientDialTimeoutKey       = "etc.network.ws.client.dialTimeout"
	defaultClientReadBufferSizeKey    = "etc.network.ws.client.readBufferSize"
	defaultClientWriteTimeoutKey      = "etc.network.ws.client.writeTimeout"
	defaultClientWriteQueueSizeKey    = "etc.network.ws.client.writeQueueSize"
	defaultClientHeartbeatIntervalKey = "etc.network.ws.client.heartbeatInterval"
	defaultClientCloseTimeoutKey      = "etc.network.ws.client.closeTimeout"
	defaultClientEnableCompressionKey = "etc.network.ws.client.enableCompression"
	defaultClientCompressionLevelKey  = "etc.network.ws.client.compressionLevel"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	url               string        // Dial URL
	tlsConfig         *tls.Config   // TLS config
	dialTimeout       time.Duration // Dial timeout, defaults to 3s
	readBufferSize    int           // Read buffer size, defaults to 4096
	writeTimeout      time.Duration // Write timeout, defaults to no timeout
	writeQueueSize    int           // Write queue size, defaults to 1024
	heartbeatInterval time.Duration // Heartbeat interval, defaults to 10s
	closeTimeout      time.Duration // Graceful close timeout, defaults to 0s (no limit)
	enableCompression bool          // Whether to enable compression, defaults to false
	compressionLevel  int           // Compression level, defaults to 1
}

// defaultClientOptions builds the default client options.
//
// It reads the environment config (etc.network.ws.client.*) first and falls back to the built-in
// defaults when a value is missing.
func defaultClientOptions() *clientOptions {
	opts := &clientOptions{}
	opts.enableCompression = etc.Get(defaultClientEnableCompressionKey, defaultClientEnableCompression).Bool()

	if url := etc.Get(defaultClientUrlKey, defaultClientUrl).String(); url != "" {
		opts.url = url
	} else {
		opts.url = defaultClientUrl
	}

	if dialTimeout := etc.Get(defaultClientDialTimeoutKey, defaultClientDialTimeout).Duration(); dialTimeout > 0 {
		opts.dialTimeout = dialTimeout
	} else {
		opts.dialTimeout = xconv.Duration(defaultClientDialTimeout)
	}

	if readBufferSize := etc.Get(defaultClientReadBufferSizeKey, defaultClientReadBufferSize).Int(); readBufferSize > 0 {
		opts.readBufferSize = readBufferSize
	} else {
		opts.readBufferSize = defaultClientReadBufferSize
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

	if compressionLevel := etc.Get(defaultClientCompressionLevelKey, defaultClientCompressionLevel).Int(); compressionLevel >= 1 && compressionLevel <= 9 {
		opts.compressionLevel = compressionLevel
	} else {
		opts.compressionLevel = defaultClientCompressionLevel
	}

	caFile := etc.Get(defaultClientCAFileKey).String()
	serverName := etc.Get(defaultClientServerNameKey).String()

	if caFile != "" || serverName != "" {
		if config, err := ctls.MakeTCPClientTLSConfig(caFile, serverName); err != nil {
			log.Warnf("make ws client tls config failed: %v", err)
		} else {
			opts.tlsConfig = config
		}
	}

	return opts
}

// WithClientUrl sets the dial URL.
func WithClientUrl(url string) ClientOption {
	return func(o *clientOptions) {
		if url != "" {
			o.url = url
		} else {
			log.Warnf("the specified url is empty and will be ignored")
		}
	}
}

// WithClientCredentials sets the CA certificate and the server name to verify.
func WithClientCredentials(caFile string, serverName string) ClientOption {
	return func(o *clientOptions) {
		if caFile != "" || serverName != "" {
			if config, err := ctls.MakeTCPClientTLSConfig(caFile, serverName); err != nil {
				log.Warnf("make ws client tls config failed: %v", err)
			} else {
				o.tlsConfig = config
			}
		} else {
			log.Warnf("the specified caFile or serverName is empty and will be ignored")
		}
	}
}

// WithClientTLSConfig sets the TLS config.
func WithClientTLSConfig(tlsConfig *tls.Config) ClientOption {
	return func(o *clientOptions) {
		o.tlsConfig = tlsConfig
	}
}

// WithClientDialTimeout sets the dial timeout.
func WithClientDialTimeout(dialTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if dialTimeout >= 0 {
			o.dialTimeout = dialTimeout
		} else {
			log.Warnf("the specified dialTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientReadBufferSize sets the read buffer size.
func WithClientReadBufferSize(readBufferSize int) ClientOption {
	return func(o *clientOptions) {
		if readBufferSize > 0 {
			o.readBufferSize = readBufferSize
		} else {
			log.Warnf("the specified readBufferSize is less than zero and will be ignored")
		}
	}
}

// WithClientWriteTimeout sets the write timeout.
func WithClientWriteTimeout(writeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientWriteQueueSize sets the write queue size.
func WithClientWriteQueueSize(writeQueueSize int) ClientOption {
	return func(o *clientOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
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

// WithClientCloseTimeout sets the graceful close timeout.
//
// When the write queue has not drained before the timeout elapses, the wait is abandoned and the
// connection is closed forcibly. The default value of 0 means no limit.
func WithClientCloseTimeout(closeTimeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		if closeTimeout >= 0 {
			o.closeTimeout = closeTimeout
		} else {
			log.Warnf("the specified closeTimeout is less than zero and will be ignored")
		}
	}
}

// WithClientCompression sets whether to enable compression.
func WithClientCompression(enableCompression bool) ClientOption {
	return func(o *clientOptions) { o.enableCompression = enableCompression }
}

// WithClientCompressionLevel sets the compression level.
func WithClientCompressionLevel(compressionLevel int) ClientOption {
	return func(o *clientOptions) {
		if compressionLevel >= 1 && compressionLevel <= 9 {
			o.compressionLevel = compressionLevel
		} else {
			log.Warnf("the specified compressionLevel is out of range and will be ignored")
		}
	}
}
