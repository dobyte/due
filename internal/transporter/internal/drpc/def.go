package drpc

import "time"

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

const (
	connClosed int32 = iota // 连接关闭
	connOpened              // 连接打开
	connHanged              // 连接挂起
)

const (
	defaultHeartbeatInterval = 10 * time.Second // 心跳间隔时间
)
