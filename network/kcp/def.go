package kcp

const protocol = "kcp"

const (
	closeSig   int = iota // 关闭信号
	dataPacket            // 数据包
)

type chWrite struct {
	typ     int
	msg     []byte
	msgType int
}
