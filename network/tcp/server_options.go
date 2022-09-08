package tcp

import "time"

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr              string        // 监听地址，默认0.0.0.0:3553
	maxConnNum        int           // 最大连接数，默认5000
	maxMsgLength      int           // 最大消息长度，默认1K
	heartbeatInterval time.Duration // 心跳间隔，默认10s
}

// WithServerListenAddr 设置监听地址
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerMaxConnNum 设置连接的最大连接数
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerMaxMsgLength 设置消息最大长度
func WithServerMaxMsgLength(maxMsgLength int) ServerOption {
	return func(o *serverOptions) { o.maxMsgLength = maxMsgLength }
}

// WithServerHeartbeatInterval 设置心跳间隔时间
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatInterval = heartbeatInterval }
}
