package server

import "time"

type Options struct {
	Addr           string        // 监听地址
	Expose         bool          // 是否暴露公网IP
	WriteTimeout   time.Duration // 写超时时间
	WriteQueueSize int32         // 写队列大小
}
