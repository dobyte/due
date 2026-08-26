package kcp

// protocol 协议名称
const protocol = "kcp"

// 任务类型
const (
	closeSig        int8 = iota // 关闭信号
	dataPacket                  // 数据包
	heartbeatPacket             // 心跳包
)

// task 写入任务
// 用于在写入队列中传递待发送的消息
type task struct {
	typ int8   // 任务类型
	msg []byte // 待发送的消息字节
}
