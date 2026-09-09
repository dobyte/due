package def

const (
	B8 = 1 << iota
	B16
	B32
	B64
)

const (
	SizeBytes   = B32 // 包长度字节数
	HeaderBytes = B8  // 头信息字节数
	SeqBytes    = B64 // 序列号字节数
	RouteBytes  = B8  // 路由号字节数
	CodeBytes   = B16 // 错误码字节数
)

// MaxFrameSize 单帧最大长度（含4字节size字段）
const MaxFrameSize = 16 << 20 // 16MB

// MinFrameSize 非心跳帧的最小长度（size字段 + header + route + seq）
const MinFrameSize = RouteBytes + SeqBytes

const (
	DataBit       uint8 = 0 << 7 // 数据标识位
	HeartbeatBit  uint8 = 1 << 7 // 心跳标识位
	DisconnectBit uint8 = 1 << 6 // 断连标识位
)
