package drpcs

import "time"

const (
	connClosed int32 = iota // 连接关闭
	connOpened              // 连接打开
	connAlived              // 连接存活
	connHanged              // 连接挂起
)

const (
	heartbeatInterval = 10 * time.Second // 心跳间隔时间
	maxBatchWriteNum  = 64               // 最大批量写入消息数量
)
