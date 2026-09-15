package ws

import (
	"net/http"
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
	defaultServerEnableCompression  = false
	defaultServerCompressionLevel   = 1
)

const (
	defaultServerAddrKey                = "etc.network.ws.server.addr"
	defaultServerPathKey                = "etc.network.ws.server.path"
	defaultServerCheckOriginsKey        = "etc.network.ws.server.origins"
	defaultServerKeyFileKey             = "etc.network.ws.server.keyFile"
	defaultServerCertFileKey            = "etc.network.ws.server.certFile"
	defaultServerMaxConnNumKey          = "etc.network.ws.server.maxConnNum"
	defaultServerReadBufferSizeKey      = "etc.network.ws.server.readBufferSize"
	defaultServerWriteBufferSizeKey     = "etc.network.ws.server.writeBufferSize"
	defaultServerWriteTimeoutKey        = "etc.network.ws.server.writeTimeout"
	defaultServerWriteQueueSizeKey      = "etc.network.ws.server.writeQueueSize"
	defaultServerHeartbeatIntervalKey   = "etc.network.ws.server.heartbeatInterval"
	defaultServerHeartbeatMechanismKey  = "etc.network.ws.server.heartbeatMechanism"
	defaultServerAuthorizeTimeoutKey    = "etc.network.ws.server.authorizeTimeout"
	defaultServerEnableCompressionKey   = "etc.network.ws.server.enableCompression"
	defaultServerCompressionLevelKey    = "etc.network.ws.server.compressionLevel"
	defaultServerEnableProxyProtocolKey = "etc.network.ws.server.enableProxyProtocol"
)

const (
	RespHeartbeat HeartbeatMechanism = "resp" // 响应式心跳
	TickHeartbeat HeartbeatMechanism = "tick" // 主动定时心跳
)

type HeartbeatMechanism string

type ServerOption func(o *serverOptions)

type CheckOriginFunc func(r *http.Request) bool

type serverOptions struct {
	addr                string             // 监听地址
	maxConnNum          int                // 最大连接数
	certFile            string             // 证书文件
	keyFile             string             // 秘钥文件
	path                string             // 路径，默认为"/"
	checkOrigin         CheckOriginFunc    // 跨域检测
	readBufferSize      int                // 读缓冲区大小，默认4096
	writeBufferSize     int                // 写缓冲区大小，默认4096
	writeTimeout        time.Duration      // 写入超时时间，默认无超时
	writeQueueSize      int                // 写入队列大小，默认1024
	heartbeatInterval   time.Duration      // 心跳间隔时间，默认10s
	heartbeatMechanism  HeartbeatMechanism // 心跳机制，默认resp
	authorizeTimeout    time.Duration      // 授权超时时间，默认0s，不检测
	enableCompression   bool               // 是否开启压缩，默认false
	compressionLevel    int                // 压缩等级，默认1
	enableProxyProtocol bool               // 是否开启代理协议，默认false
}

// defaultServerOptions 构建默认服务器配置
// 优先读取环境配置（etc.network.ws.server.*），缺失时回退到内置默认值
// @return @1 *serverOptions 服务器配置
func defaultServerOptions() *serverOptions {
	opts := &serverOptions{}
	opts.path = etc.Get(defaultServerPathKey, defaultServerPath).String()
	opts.certFile = etc.Get(defaultServerCertFileKey).String()
	opts.keyFile = etc.Get(defaultServerKeyFileKey).String()
	opts.enableProxyProtocol = etc.Get(defaultServerEnableProxyProtocolKey).Bool()
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

	if compressionLevel := etc.Get(defaultServerCompressionLevelKey, defaultServerCompressionLevel).Int(); compressionLevel >= 1 && compressionLevel <= 9 {
		opts.compressionLevel = compressionLevel
	} else {
		opts.compressionLevel = defaultServerCompressionLevel
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

// WithServerAddr 设置监听地址
// @param addr string 监听地址
// @return @1 ServerOption 服务器配置项
func WithServerAddr(addr string) ServerOption {
	return func(o *serverOptions) {
		if addr != "" {
			o.addr = addr
		} else {
			log.Warnf("the specified addr is empty and will be ignored")
		}
	}
}

// WithServerPath 设置Websocket的连接路径
// @param path string 连接路径
// @return @1 ServerOption 服务器配置项
func WithServerPath(path string) ServerOption {
	return func(o *serverOptions) { o.path = path }
}

// WithServerCredentials 设置服务器证书和秘钥
// @param certFile string 证书文件
// @param keyFile string 秘钥文件
// @return @1 ServerOption 服务器配置项
func WithServerCredentials(certFile, keyFile string) ServerOption {
	return func(o *serverOptions) {
		if certFile != "" && keyFile != "" {
			o.certFile, o.keyFile = certFile, keyFile
		} else {
			log.Warnf("the specified certFile or keyFile is empty and will be ignored")
		}
	}
}

// WithServerCheckOrigin 设置Websocket跨域检测函数
// @param checkOrigin CheckOriginFunc 跨域检测函数
// @return @1 ServerOption 服务器配置项
func WithServerCheckOrigin(checkOrigin CheckOriginFunc) ServerOption {
	return func(o *serverOptions) { o.checkOrigin = checkOrigin }
}

// WithServerMaxConnNum 设置连接的最大连接数
// @param maxConnNum int 最大连接数
// @return @1 ServerOption 服务器配置项
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) {
		if maxConnNum > 0 {
			o.maxConnNum = maxConnNum
		} else {
			log.Warnf("the specified maxConnNum is less than zero and will be ignored")
		}
	}
}

// WithServerReadBufferSize 设置读取缓冲区大小
// @param readBufferSize int 读取缓冲区大小
// @return @1 ServerOption 服务器配置项
func WithServerReadBufferSize(readBufferSize int) ServerOption {
	return func(o *serverOptions) {
		if readBufferSize > 0 {
			o.readBufferSize = readBufferSize
		} else {
			log.Warnf("the specified readBufferSize is less than zero and will be ignored")
		}
	}
}

// WithServerWriteBufferSize 设置写入缓冲区大小
// @param writeBufferSize int 写入缓冲区大小
// @return @1 ServerOption 服务器配置项
func WithServerWriteBufferSize(writeBufferSize int) ServerOption {
	return func(o *serverOptions) {
		if writeBufferSize > 0 {
			o.writeBufferSize = writeBufferSize
		} else {
			log.Warnf("the specified writeBufferSize is less than zero and will be ignored")
		}
	}
}

// WithServerWriteTimeout 设置写超时时间
// @param writeTimeout time.Duration 写超时时间
// @return @1 ServerOption 服务器配置项
func WithServerWriteTimeout(writeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if writeTimeout >= 0 {
			o.writeTimeout = writeTimeout
		} else {
			log.Warnf("the specified writeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerWriteQueueSize 设置写入队列大小
// @param writeQueueSize int 写入队列大小
// @return @1 ServerOption 服务器配置项
func WithServerWriteQueueSize(writeQueueSize int) ServerOption {
	return func(o *serverOptions) {
		if writeQueueSize > 0 {
			o.writeQueueSize = writeQueueSize
		} else {
			log.Warnf("the specified writeQueueSize is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
// @param heartbeatInterval time.Duration 心跳间隔时间
// @return @1 ServerOption 服务器配置项
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) {
		if heartbeatInterval >= 0 {
			o.heartbeatInterval = heartbeatInterval
		} else {
			log.Warnf("the specified heartbeatInterval is less than zero and will be ignored")
		}
	}
}

// WithServerHeartbeatMechanism 设置心跳机制
// @param heartbeatMechanism HeartbeatMechanism 心跳机制
// @return @1 ServerOption 服务器配置项
func WithServerHeartbeatMechanism(heartbeatMechanism HeartbeatMechanism) ServerOption {
	return func(o *serverOptions) { o.heartbeatMechanism = heartbeatMechanism }
}

// WithServerAuthorizeTimeout 设置授权超时时间
// @param authorizeTimeout time.Duration 授权超时时间
// @return @1 ServerOption 服务器配置项
func WithServerAuthorizeTimeout(authorizeTimeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		if authorizeTimeout >= 0 {
			o.authorizeTimeout = authorizeTimeout
		} else {
			log.Warnf("the specified authorizeTimeout is less than zero and will be ignored")
		}
	}
}

// WithServerEnableCompression 设置是否开启压缩
// @param enableCompression bool 是否开启压缩
// @return @1 ServerOption 服务器配置项
func WithServerEnableCompression(enableCompression bool) ServerOption {
	return func(o *serverOptions) { o.enableCompression = enableCompression }
}

// WithServerCompressionLevel 设置压缩等级
// @param compressionLevel int 压缩等级
// @return @1 ServerOption 服务器配置项
func WithServerCompressionLevel(compressionLevel int) ServerOption {
	return func(o *serverOptions) {
		if compressionLevel >= 1 && compressionLevel <= 9 {
			o.compressionLevel = compressionLevel
		} else {
			log.Warnf("the specified compressionLevel is out of range and will be ignored")
		}
	}
}

// WithServerEnableProxyProtocol 设置是否开启代理协议
// @param enableProxyProtocol bool 是否开启代理协议
// @return @1 ServerOption 服务器配置项
func WithServerEnableProxyProtocol(enableProxyProtocol bool) ServerOption {
	return func(o *serverOptions) { o.enableProxyProtocol = enableProxyProtocol }
}
