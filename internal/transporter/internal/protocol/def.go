package protocol

const (
	defaultSizeBytes   = 4 // 包长度字节数
	defaultHeaderBytes = 1 // 头信息字节数
	defaultSeqBytes    = 8 // 序列号字节数
	defaultRouteBytes  = 1 // 路由号字节数
	defaultCodeBytes   = 2 // 错误码字节数
)

const (
	dataBit      uint8 = 0 << 7 // 数据标识位
	heartbeatBit uint8 = 1 << 7 // 心跳标识位
)

const (
	b8 = 1 << iota
	b16
	b32
	b64
)
