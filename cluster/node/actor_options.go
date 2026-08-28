package node

import "time"

// Actor配置项
type actorOptions struct {
	id                  string        // Actor编号
	kind                string        // Actor类型
	args                []any         // 传递到Processor中的参数
	wait                bool          // 是否需要等待
	dispatch            bool          // 是否接受调度器调度
	taskQueueSize       int32         // 任务队列大小
	taskWriteTimeout    time.Duration // 任务入队超时时间
	messageQueueSize    int32         // 消息队列大小
	messageWriteTimeout time.Duration // 消息入队超时时间
}

// ActorOption Actor配置函数
type ActorOption func(o *actorOptions)

// 创建默认Actor配置项
// @return @1 *actorOptions 默认Actor配置项
func defaultActorOptions() *actorOptions {
	return &actorOptions{
		wait:                true,
		dispatch:            true,
		taskQueueSize:       1024,
		taskWriteTimeout:    3 * time.Second,
		messageQueueSize:    2048,
		messageWriteTimeout: 3 * time.Second,
	}
}

// WithActorID 设置Actor编号
// @param id string Actor编号
// @return @1 ActorOption Actor配置项
func WithActorID(id string) ActorOption {
	return func(o *actorOptions) { o.id = id }
}

// WithActorKind 设置Actor类型
// @param kind string Actor类型
// @return @1 ActorOption Actor配置项
func WithActorKind(kind string) ActorOption {
	return func(o *actorOptions) { o.kind = kind }
}

// WithActorArgs 设置传递到Processor中的参数
// @param args ...any Processor参数
// @return @1 ActorOption Actor配置项
func WithActorArgs(args ...any) ActorOption {
	return func(o *actorOptions) { o.args = append(o.args, args...) }
}

// WithActorNonWait 设置Actor无需等待属性（Node组件关关闭时无需等待此Actor结束）
// @return @1 ActorOption Actor配置项
func WithActorNonWait() ActorOption {
	return func(o *actorOptions) { o.wait = false }
}

// WithActorNonDispatch 设置Actor不可调度
// @return @1 ActorOption Actor配置项
func WithActorNonDispatch() ActorOption {
	return func(o *actorOptions) { o.dispatch = false }
}

// WithActorTaskQueueSize 设置任务队列大小
// @param size int32 任务队列大小
// @return @1 ActorOption Actor配置项
func WithActorTaskQueueSize(size int32) ActorOption {
	return func(o *actorOptions) { o.taskQueueSize = size }
}

// WithActorTaskWriteTimeout 设置任务入队超时时间
// @param timeout time.Duration 任务入队超时时间
// @return @1 ActorOption Actor配置项
func WithActorTaskWriteTimeout(timeout time.Duration) ActorOption {
	return func(o *actorOptions) { o.taskWriteTimeout = timeout }
}

// WithActorMessageQueueSize 设置消息队列大小
// @param size int32 消息队列大小
// @return @1 ActorOption Actor配置项
func WithActorMessageQueueSize(size int32) ActorOption {
	return func(o *actorOptions) { o.messageQueueSize = size }
}

// WithActorMessageWriteTimeout 设置消息入队超时时间
// @param timeout time.Duration 消息入队超时时间
// @return @1 ActorOption Actor配置项
func WithActorMessageWriteTimeout(timeout time.Duration) ActorOption {
	return func(o *actorOptions) { o.messageWriteTimeout = timeout }
}
