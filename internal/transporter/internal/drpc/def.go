package drpc

import (
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
)

const (
	connClosed int32 = iota // 连接关闭
	connOpened              // 连接打开
	connAlived              // 连接存活
	connHanged              // 连接挂起
)

const (
	maxBatchWriteNum = 64              // 最大批量写入消息数量
	maxRetentionTime = 1 * time.Second // 最大保留时间
)

// heartbeatInterval 心跳间隔（原子访问，单位：纳秒；便于测试注入小间隔验证心跳与超时行为）
var heartbeatInterval atomic.Int64

func init() {
	heartbeatInterval.Store(int64(10 * time.Second))
}

// closedQueue 已关闭队列
type closedQueue struct {
	time  time.Time                   // 关闭时间
	queue *queue.Queue[buffer.Buffer] // 消息队列
}

type RouteHandler func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error
