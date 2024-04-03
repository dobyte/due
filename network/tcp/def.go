package tcp

const protocol = "tcp"

const (
	closeSig   int = iota // 关闭信号
	dataPacket            // 数据包
)

type chWrite struct {
	typ int
	msg []byte
}
