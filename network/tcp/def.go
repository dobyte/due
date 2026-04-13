package tcp

const protocol = "tcp"

const (
	closeSig   int8 = iota // 关闭信号
	dataPacket             // 数据包
)

type task struct {
	typ int8
	msg []byte
}
