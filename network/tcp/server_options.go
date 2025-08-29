package tcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultServerAddr               = ":3553"
	defaultServerMaxConnNum         = 5000
	defaultServerHeartbeatInterval  = "10s"
	defaultServerHeartbeatMechanism = "resp"
	defaultServerAuthorizeTimeout   = "0s"
)

const (
	defaultServerAddrKey               = "etc.network.tcp.server.addr"
	defaultServerCertFileKey           = "etc.network.tcp.server.certFile"
	defaultServerKeyFileKey            = "etc.network.tcp.server.keyFile"
	defaultServerMaxConnNumKey         = "etc.network.tcp.server.maxConnNum"
	defaultServerHeartbeatIntervalKey  = "etc.network.tcp.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey = "etc.network.tcp.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey   = "etc.network.tcp.server.authorizeTimeout"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // 响应式心跳
	TickHeartbeat HeartbeatMechanism = "tick" // 主动定时心跳
)

type HeartbeatMechanism string

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr               string             // 监听地址，默认0.0.0.0:3553
	certFile           string             // 证书文件
	keyFile            string             // 秘钥文件
	maxConnNum         int                // 最大连接数，默认5000
	heartbeatInterval  time.Duration      // 心跳检测间隔时间，默认10s
	heartbeatMechanism HeartbeatMechanism // 心跳机制，默认resp
	authorizeTimeout   time.Duration      // 授权超时时间，默认0s，不检测
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		addr:               etc.Get(defaultServerAddrKey, defaultServerAddr).String(),
		certFile:           etc.Get(defaultServerCertFileKey).String(),
		keyFile:            etc.Get(defaultServerKeyFileKey).String(),
		maxConnNum:         etc.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		heartbeatInterval:  etc.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration(),
		heartbeatMechanism: HeartbeatMechanism(etc.Get(defaultServerHeartbeatMechanismKey, defaultServerHeartbeatMechanism).String()),
		authorizeTimeout:   etc.Get(defaultServerAuthorizeTimeoutKey, defaultServerAuthorizeTimeout).Duration(),
	}
}

// WithServerListenAddr 设置监听地址
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerCredentials 设置服务器证书和秘钥
func WithServerCredentials(certFile, keyFile string) ServerOption {
	return func(o *serverOptions) { o.certFile, o.keyFile = certFile, keyFile }
}

// WithServerMaxConnNum 设置连接的最大连接数
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithServerHeartbeatMechanism 设置心跳机制
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) { o.heartbeatMechanism = heartbeatMechanism }
}

// WithServerAuthorizeTimeout 设置授权超时时间
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) { o.authorizeTimeout = authorizeTimeout }
}
