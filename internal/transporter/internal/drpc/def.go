package drpc

import "time"

const (
	connClosed int32 = iota // 连接关闭
	connOpened              // 连接打开
	connHanged              // 连接挂起
)

const (
	defaultHeartbeatInterval = 10 * time.Second // 心跳间隔时间
	defaultDialTimeout       = 3 * time.Second  // 默认拨号/握手超时时间
	replyCacheTTL            = time.Minute      // 响应缓存过期时间
	replyWaitTimeout         = time.Minute      // 等待执行中请求完成的最长时间，防止等待协程泄漏
)
