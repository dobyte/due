package ws

import (
	"github.com/symsimmy/due/config"
	"net/http"
	"time"
)

const (
	defaultServerAddr                   = ":3553"
	defaultServerPath                   = "/"
	defaultServerMaxMsgLen              = 1024
	defaultServerMaxConnNum             = 5000
	defaultServerCheckOrigin            = "*"
	defaultServerHeartbeatCheck         = false
	defaultServerHeartbeatCheckInterval = 10
	defaultServerHandshakeTimeout       = 10
	defaultServerMsgType                = binaryMessage
)

const (
	defaultServerAddrKey                   = "config.network.ws.server.addr"
	defaultServerPathKey                   = "config.network.ws.server.path"
	defaultServerMaxMsgLenKey              = "config.network.ws.server.maxMsgLen"
	defaultServerMaxConnNumKey             = "config.network.ws.server.maxConnNum"
	defaultServerCheckOriginsKey           = "config.network.ws.server.origins"
	defaultServerKeyFileKey                = "config.network.ws.server.keyFile"
	defaultServerCertFileKey               = "config.network.ws.server.certFile"
	defaultServerHeartbeatCheckKey         = "config.network.ws.server.heartbeatCheck"
	defaultServerHeartbeatCheckIntervalKey = "config.network.ws.server.heartbeatCheckInterval"
	defaultServerHandshakeTimeoutKey       = "config.network.ws.server.handshakeTimeout"
	defaultServerMsgTypeKey                = "config.network.ws.server.msgType"
)

type ServerOption func(o *serverOptions)

type CheckOriginFunc func(r *http.Request) bool

type serverOptions struct {
	addr                   string          // 监听地址
	maxMsgLen              int             // 最大消息长度（字节），默认1kb
	maxConnNum             int             // 最大连接数
	certFile               string          // 证书文件
	keyFile                string          // 秘钥文件
	path                   string          // 路径，默认为"/"
	msgType                string          // 默认消息类型，text | binary
	checkOrigin            CheckOriginFunc // 跨域检测
	enableHeartbeatCheck   bool            // 是否启用心跳检测
	heartbeatCheckInterval time.Duration   // 心跳检测间隔时间，默认10s
	handshakeTimeout       time.Duration   // 握手超时时间，默认10s
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
		addr:                   config.Get(defaultServerAddrKey, defaultServerAddr).String(),
		maxMsgLen:              config.Get(defaultServerMaxMsgLenKey, defaultServerMaxMsgLen).Int(),
		maxConnNum:             config.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		path:                   config.Get(defaultServerPathKey, defaultServerPath).String(),
		checkOrigin:            checkOrigin,
		keyFile:                config.Get(defaultServerKeyFileKey).String(),
		certFile:               config.Get(defaultServerCertFileKey).String(),
		msgType:                config.Get(defaultServerMsgTypeKey, defaultServerMsgType).String(),
		enableHeartbeatCheck:   config.Get(defaultServerHeartbeatCheckKey, defaultServerHeartbeatCheck).Bool(),
		heartbeatCheckInterval: config.Get(defaultServerHeartbeatCheckIntervalKey, defaultServerHeartbeatCheckInterval).Duration() * time.Second,
		handshakeTimeout:       config.Get(defaultServerHandshakeTimeoutKey, defaultServerHandshakeTimeout).Duration() * time.Second,
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

// WithServerMsgType 设置默认消息类型
func WithServerMsgType(msgType string) ServerOption {
	return func(o *serverOptions) { o.msgType = msgType }
}

// WithServerCheckOrigin 设置Websocket跨域检测函数
func WithServerCheckOrigin(checkOrigin CheckOriginFunc) ServerOption {
	return func(o *serverOptions) { o.checkOrigin = checkOrigin }
}

// WithServerEnableHeartbeatCheck 是否启用心跳检测
func WithServerEnableHeartbeatCheck(enable bool) ServerOption {
	return func(o *serverOptions) { o.enableHeartbeatCheck = enable }
}

// WithServerHeartbeatCheckInterval 设置心跳检测间隔时间
func WithServerHeartbeatCheckInterval(heartbeatCheckInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatCheckInterval = heartbeatCheckInterval }
}

// WithServerHandshakeTimeout 设置握手超时时间
func WithServerHandshakeTimeout(handshakeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) { o.handshakeTimeout = handshakeTimeout }
}
