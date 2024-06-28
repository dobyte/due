package ws

const protocol = "ws"

const (
	closeSig        int = iota // 关闭信号
	dataPacket                 // 数据包
	heartbeatPacket            // 心跳包
)

type chWrite struct {
	typ int
	msg []byte
}
