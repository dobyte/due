package ws

// protocol 协议标识
const protocol = "ws"

// 任务类型
const (
	closeSig        int8 = iota // 关闭信号
	dataPacket                  // 数据包
	heartbeatPacket             // 心跳包
)

// task 写入队列的任务对象
// 由对象池复用，typ 标识任务类型，msg 为待发送的消息字节
type task struct {
	typ int8
	msg []byte
}
