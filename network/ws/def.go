package ws

const protocol = "ws"

const (
	closeSig        int8 = iota // 关闭信号
	dataPacket                  // 数据包
	heartbeatPacket             // 心跳包
)

type task struct {
	typ int8
	msg []byte
}
