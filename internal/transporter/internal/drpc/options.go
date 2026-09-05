package drpc

import (
	"time"

	"github.com/dobyte/due/v2/cluster"
)

type ServerOptions struct {
	Addr           string        // 监听地址
	Expose         bool          // 是否暴露公网IP
	WriteTimeout   time.Duration // 写超时时间
	WriteQueueSize int32         // 写队列大小
}

type ClientOptions struct {
	ID                string        // 实例ID
	Kind              cluster.Kind  // 实例类型
	Addr              string        //
	ConnNum           int           // 连接数
	CallTimeout       time.Duration // 调用超时时间
	DialTimeout       time.Duration // 拨号超时时间
	DialRetryTimes    int           // 拨号重试次数
	WriteTimeout      time.Duration // 写超时时间
	WriteQueueSize    int32         // 写队列大小
	FaultRecoveryTime time.Duration // 故障恢复时间
}
