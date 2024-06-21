package def

import "time"

const (
	ConnClosed   int32 = iota // 连接打开
	ConnOpened                // 连接关闭
	ConnRetrying              // 连接重试
)

const (
	HeartbeatInterval = 10 * time.Second // 心跳间隔时间
)
