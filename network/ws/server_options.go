package ws

import (
	"github.com/dobyte/due/v2/config"
	"net/http"
	"time"
)

const (
	defaultServerAddr              = ":3553"
	defaultServerPath              = "/"
	defaultServerMaxConnNum        = 5000
	defaultServerCheckOrigin       = "*"
	defaultServerHeartbeatInterval = 10
	defaultServerHandshakeTimeout  = 10
)

const (
	defaultServerAddrKey              = "config.network.ws.server.addr"
	defaultServerPathKey              = "config.network.ws.server.path"
	defaultServerMaxConnNumKey        = "config.network.ws.server.maxConnNum"
	defaultServerCheckOriginsKey      = "config.network.ws.server.origins"
	defaultServerKeyFileKey           = "config.network.ws.server.keyFile"
	defaultServerCertFileKey          = "config.network.ws.server.certFile"
	defaultServerHeartbeatIntervalKey = "config.network.ws.server.heartbeatInterval"
	defaultServerHandshakeTimeoutKey  = "config.network.ws.server.handshakeTimeout"
)

type ServerOption func(o *serverOptions)

type CheckOriginFunc func(r *http.Request) bool

type serverOptions struct {
	addr              string          // 监听地址
	maxConnNum        int             // 最大连接数
	certFile          string          // 证书文件
	keyFile           string          // 秘钥文件
	path              string          // 路径，默认为"/"
	checkOrigin       CheckOriginFunc // 跨域检测
	heartbeatInterval time.Duration   // 心跳检测间隔时间，默认10s
	handshakeTimeout  time.Duration   // 握手超时时间，默认10s
}

func defaultServerOptions() *serverOptions {
	origins := config.Get(defaultServerCheckOriginsKey, []string{defaultServerCheckOrigin}).Strings()
	checkOrigin := func(r *http.Request) bool {
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

	return &serverOptions{
		addr:              config.Get(defaultServerAddrKey, defaultServerAddr).String(),
		maxConnNum:        config.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		path:              config.Get(defaultServerPathKey, defaultServerPath).String(),
		checkOrigin:       checkOrigin,
		keyFile:           config.Get(defaultServerKeyFileKey).String(),
		certFile:          config.Get(defaultServerCertFileKey).String(),
		heartbeatInterval: config.Get(defaultServerHeartbeatIntervalKey, defaultServerHeartbeatInterval).Duration() * time.Second,
		handshakeTimeout:  config.Get(defaultServerHandshakeTimeoutKey, defaultServerHandshakeTimeout).Duration() * time.Second,
	}
}

// WithServerListenAddr 设置监听地址
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerMaxConnNum 设置连接的最大连接数
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerPath 设置Websocket的连接路径
func WithServerPath(path string) ServerOption {
	return func(o *serverOptions) { o.path = path }
}

// WithServerCredentials 设置证书和秘钥
func WithServerCredentials(certFile, keyFile string) ServerOption {
	return func(o *serverOptions) { o.keyFile, o.certFile = keyFile, certFile }
}

// WithServerCheckOrigin 设置Websocket跨域检测函数
func WithServerCheckOrigin(checkOrigin CheckOriginFunc) ServerOption {
	return func(o *serverOptions) { o.checkOrigin = checkOrigin }
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatInterval = heartbeatInterval }
}

// WithServerHandshakeTimeout 设置握手超时时间
func WithServerHandshakeTimeout(handshakeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) { o.handshakeTimeout = handshakeTimeout }
}
