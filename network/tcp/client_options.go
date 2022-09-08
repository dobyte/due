package tcp

import "time"

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	maxMsgLength      int           // 最大消息长度
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

// WithClientDialAddr 设置拨号地址
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientMaxMsgLength 设置消息最大长度
func WithClientMaxMsgLength(maxMsgLength int) ClientOption {
	return func(o *clientOptions) { o.maxMsgLength = maxMsgLength }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
