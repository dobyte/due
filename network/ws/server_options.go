package ws

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultServerAddr               = ":3553"
	defaultServerPath               = "/"
	defaultServerMaxConnNum         = 5000
	defaultServerCheckOrigin        = "*"
	defaultServerReadBufferSize     = 4096
	defaultServerWriteBufferSize    = 4096
	defaultServerWriteTimeout       = "0s"
	defaultServerWriteQueueSize     = 1024
	defaultServerHeartbeatInterval  = "10s"
	defaultServerHeartbeatMechanism = "resp"
	defaultServerAuthorizeTimeout   = "0s"
	defaultServerCloseTimeout       = "0s"
	defaultServerEnableCompression  = false
	defaultServerCompressionLevel   = 1
	defaultServerProxyMode          = ProxyModeNone
)

const (
	defaultServerAddrKey               = "etc.network.ws.server.addr"
	defaultServerPathKey               = "etc.network.ws.server.path"
	defaultServerCheckOriginsKey       = "etc.network.ws.server.origins"
	defaultServerKeyFileKey            = "etc.network.ws.server.keyFile"
	defaultServerCertFileKey           = "etc.network.ws.server.certFile"
	defaultServerMaxConnNumKey         = "etc.network.ws.server.maxConnNum"
	defaultServerReadBufferSizeKey     = "etc.network.ws.server.readBufferSize"
	defaultServerWriteBufferSizeKey    = "etc.network.ws.server.writeBufferSize"
	defaultServerWriteTimeoutKey       = "etc.network.ws.server.writeTimeout"
	defaultServerWriteQueueSizeKey     = "etc.network.ws.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey  = "etc.network.ws.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey = "etc.network.ws.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey   = "etc.network.ws.server.authorizeTimeout"
	defaultServerCloseTimeoutKey       = "etc.network.ws.server.closeTimeout"
	defaultServerEnableCompressionKey  = "etc.network.ws.server.enableCompression"
	defaultServerCompressionLevelKey   = "etc.network.ws.server.compressionLevel"
	defaultServerProxyModeKey          = "etc.network.ws.server.proxyMode"
	defaultServerProxyOptionsKey       = "etc.network.ws.server.proxyOptions"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // Responsive heartbeat
	TickHeartbeat HeartbeatMechanism = "tick" // Active tick heartbeat
)

type HeartbeatMechanism string

const (
	ProxyModeNone        ProxyMode = iota // No proxy mode
	ProxyModeTransport                    // Transport mode (layer 4 proxy, the server enables the proxy protocol)
	ProxyModeApplication                  // Application mode (layer 7 proxy)
)

type ProxyMode int

type ServerOption func(o *serverOptions)

type CheckOriginFunc func(r *http.Request) bool

type ProxyOptions struct {
	ProxyHeader string            `json:"proxyHeader"` // Client IP header, defaults to "X-Forwarded-For"
	PortHeader  string            `json:"portHeader"`  // Client port header, defaults to "X-Forwarded-Port"
	TrustProxy  TrustProxyOptions `json:"trustProxy"`  // Trust proxy config
}

type TrustProxyOptions struct {
	Enable    bool                `json:"enable"`    // Whether to trust proxies, defaults to false
	Proxies   []string            `json:"proxies"`   // List of trusted proxy IP addresses or CIDR ranges
	LinkLocal bool                `json:"linkLocal"` // Whether to trust all link-local IP ranges (e.g. 169.254.0.0/16, fe80::/10)
	Loopback  bool                `json:"loopback"`  // Whether to trust all loopback IP ranges (e.g. 127.0.0.0/8, ::1/128)
	Private   bool                `json:"private"`   // Whether to trust all private IP ranges (e.g. 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
	ips       map[string]struct{} `json:"-"`         // Trusted proxy IP address set
	ranges    []*net.IPNet        `json:"-"`         // Trusted proxy IP range set
}

type serverOptions struct {
	addr               string             // Listen address
	maxConnNum         int                // Maximum number of connections
	certFile           string             // Certificate file
	keyFile            string             // Key file
	path               string             // Path, defaults to "/"
	checkOrigin        CheckOriginFunc    // Origin check
	readBufferSize     int                // Read buffer size, defaults to 4096
	writeBufferSize    int                // Write buffer size, defaults to 4096
	writeTimeout       time.Duration      // Write timeout, defaults to no timeout
	writeQueueSize     int                // Write queue size, defaults to 1024
	heartbeatInterval  time.Duration      // Heartbeat interval, defaults to 10s
	heartbeatMechanism HeartbeatMechanism // Heartbeat mechanism, defaults to resp
	authorizeTimeout   time.Duration      // Authorization timeout, defaults to 0s (no check)
	closeTimeout       time.Duration      // Graceful close timeout, defaults to 0s (no limit)
	enableCompression  bool               // Whether to enable compression, defaults to false
	compressionLevel   int                // Compression level, defaults to 1
	proxyMode          ProxyMode          // Proxy mode, defaults to ProxyModeNone
	proxyOpts          ProxyOptions       // Proxy options, only effective when proxyMode is ProxyModeApplication
}

// defaultServerOptions builds the default server options.
//
// It reads the environment config (etc.network.ws.server.*) first and falls back to the built-in
// defaults when a value is missing.
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{}
	opts.path = etc.Get(defaultServerPathKey, defaultServerPath).String()
	opts.certFile = etc.Get(defaultServerCertFileKey).String()
	opts.keyFile = etc.Get(defaultServerKeyFileKey).String()
	opts.enableCompression = etc.Get(defaultServerEnableCompressionKey, defaultServerEnableCompression).Bool()

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

	if writeBufferSize := etc.Get(defaultServerWriteBufferSizeKey, defaultServerWriteBufferSize).Int(); writeBufferSize > 0 {
		opts.writeBufferSize = writeBufferSize
	} else {
		opts.writeBufferSize = defaultServerWriteBufferSize
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

	if compressionLevel := etc.Get(defaultServerCompressionLevelKey, defaultServerCompressionLevel).Int(); compressionLevel >= 1 && compressionLevel <= 9 {
		opts.compressionLevel = compressionLevel
	} else {
		opts.compressionLevel = defaultServerCompressionLevel
	}

	switch proxyMode := ProxyMode(etc.Get(defaultServerProxyModeKey, defaultServerProxyMode).Int()); proxyMode {
	case ProxyModeNone, ProxyModeTransport, ProxyModeApplication:
		opts.proxyMode = proxyMode
	default:
		opts.proxyMode = defaultServerProxyMode
	}

	if opts.proxyMode != ProxyModeNone {
		proxyOpts := &ProxyOptions{}

		if err := etc.Get(defaultServerProxyOptionsKey).Scan(&proxyOpts); err != nil {
			log.Warnf("scan proxy options failed: %v", err)
		} else {
			opts.proxyOpts = ProxyOptions{
				ProxyHeader: proxyOpts.ProxyHeader,
				PortHeader:  proxyOpts.PortHeader,
				TrustProxy:  handleTrustedProxy(proxyOpts.TrustProxy),
			}
		}
	}

	origins := etc.Get(defaultServerCheckOriginsKey, []string{defaultServerCheckOrigin}).Strings()
	opts.checkOrigin = func(r *http.Request) bool {
		if len(origins) == 0 {
			return false
		}

		origin := r.Header.Get("Origin")
		for _, v := range origins {
			if v == defaultServerCheckOrigin || origin == v {
				return true
			}
		}

		return false
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

// WithServerPath sets the WebSocket connection path.
func WithServerPath(path string) ServerOption {
	return func(o *serverOptions) { o.path = path }
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

// WithServerCheckOrigin sets the WebSocket origin check function.
func WithServerCheckOrigin(checkOrigin CheckOriginFunc) ServerOption {
	return func(o *serverOptions) { o.checkOrigin = checkOrigin }
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

// WithServerWriteBufferSize sets the write buffer size.
func WithServerWriteBufferSize(writeBufferSize int) ServerOption {
	return func(o *serverOptions) {
		if writeBufferSize > 0 {
			o.writeBufferSize = writeBufferSize
		} else {
			log.Warnf("the specified writeBufferSize is less than zero and will be ignored")
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
		switch heartbeatMechanism {
		case RespHeartbeat, TickHeartbeat:
			o.heartbeatMechanism = heartbeatMechanism
		default:
			log.Warnf("the specified heartbeatMechanism is invalid and will be ignored")
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

// WithServerEnableCompression sets whether to enable compression.
func WithServerEnableCompression(enableCompression bool) ServerOption {
	return func(o *serverOptions) { o.enableCompression = enableCompression }
}

// WithServerCompressionLevel sets the compression level.
func WithServerCompressionLevel(compressionLevel int) ServerOption {
	return func(o *serverOptions) {
		if compressionLevel >= 1 && compressionLevel <= 9 {
			o.compressionLevel = compressionLevel
		} else {
			log.Warnf("the specified compressionLevel is out of range and will be ignored")
		}
	}
}

// WithServerProxyMode sets the proxy mode.
func WithServerProxyMode(proxyMode ProxyMode) ServerOption {
	return func(o *serverOptions) { o.proxyMode = proxyMode }
}

// WithServerProxyOptions sets the proxy options.
func WithServerProxyOptions(proxyOpts ProxyOptions) ServerOption {
	return func(o *serverOptions) {
		o.proxyOpts = ProxyOptions{
			ProxyHeader: proxyOpts.ProxyHeader,
			PortHeader:  proxyOpts.PortHeader,
			TrustProxy:  handleTrustedProxy(proxyOpts.TrustProxy),
		}
	}
}

// handleTrustedProxy processes the trusted proxies.
func handleTrustedProxy(opts TrustProxyOptions) TrustProxyOptions {
	opts.ips = make(map[string]struct{}, len(opts.Proxies))
	opts.ranges = make([]*net.IPNet, 0, len(opts.Proxies))

	for _, proxy := range opts.Proxies {
		if strings.IndexByte(proxy, '/') >= 0 {
			if _, ipNet, err := net.ParseCIDR(proxy); err != nil {
				log.Warnf("IP range %q could not be parsed: %v", proxy, err)
			} else {
				opts.ranges = append(opts.ranges, ipNet)
			}
		} else {
			if ip := net.ParseIP(proxy); ip == nil {
				log.Warnf("IP address %q could not be parsed", proxy)
			} else {
				opts.ips[ip.String()] = struct{}{}
			}
		}
	}

	return opts
}
