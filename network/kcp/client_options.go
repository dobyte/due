package kcp

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr         string // 地址
	maxMsgLength int    // 最大消息长度
}

// WithClientDialAddr 设置拨号地址
func WithClientDialAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientMaxMsgLength 设置消息最大长度
func WithClientMaxMsgLength(maxMsgLength int) ClientOption {
	return func(o *clientOptions) { o.maxMsgLength = maxMsgLength }
}
