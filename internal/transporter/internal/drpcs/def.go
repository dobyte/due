package drpcs

import "time"

const (
	connClosed int32 = iota // 连接关闭
	connOpened              // 连接打开
	connAlived              // 连接存活
	connHanged              // 连接挂起
)

const (
	defaultHeartbeatInterval = 10 * time.Second // 心跳间隔时间
)
