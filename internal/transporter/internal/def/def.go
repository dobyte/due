package def

import "time"

const (
	ConnClosed int32 = iota // 连接关闭
	ConnOpened              // 连接打开
	ConnHanged              // 连接挂起
)

const (
	HeartbeatInterval = 10 * time.Second // 心跳间隔时间
)
