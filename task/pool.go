// Package task 提供全局任务调度池与任务组，基于 ants 协程池实现并发控制、错误聚合与优雅降级
package task

import (
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/panjf2000/ants/v2"
)

// Pool 任务池接口
type Pool interface {
	// AddTask 添加任务
	// @param task func() 待执行的任务
	// @return @1 error 错误信息
	AddTask(task func()) error
	// Release 释放任务池
	Release()
}

// globalPool 全局任务池
var globalPool Pool

// init 初始化全局任务池
func init() {
	SetPool(NewPool())
}

// defaultPool 基于 ants 的默认任务池实现
type defaultPool struct {
	pool *ants.Pool
}

// NewPool 新建任务池
// 根据配置创建 ants 协程池，支持设置池大小、非阻塞与禁用清理等特性
// @param opts ...Option 可选配置项
// @return @1 *defaultPool 任务池实例
func NewPool(opts ...Option) *defaultPool {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	p := &defaultPool{}
	p.pool, _ = ants.NewPool(o.size,
		ants.WithLogger(&logger{}),
		ants.WithNonblocking(o.nonblocking),
		ants.WithDisablePurge(o.disablePurge),
	)

	return p
}

// AddTask 添加任务
// @param task func() 待执行的任务
// @return @1 error 错误信息
func (p *defaultPool) AddTask(task func()) error {
	return p.pool.Submit(task)
}

// Release 释放任务池
func (p *defaultPool) Release() {
	p.pool.Release()
}

// SetPool 设置全局任务池
// 替换前会释放旧任务池
// @param pool Pool 任务池
func SetPool(pool Pool) {
	if globalPool != nil {
		globalPool.Release()
	}
	globalPool = pool
}

// GetPool 获取全局任务池
// @return @1 Pool 任务池
func GetPool() Pool {
	return globalPool
}

// AddTask 添加任务
// Deprecated: As of due v2.6.0+, this function simply calls [Add].
// @param task func() 待执行的任务
func AddTask(task func()) {
	if globalPool == nil {
		xcall.Go(task)
		return
	}

	if err := globalPool.AddTask(task); err != nil {
		xcall.Go(task)
		log.Warnf("add task to the task pool failed: %v", err)
		return
	}
}

// Add 执行任务
// 任务优先提交至全局任务池，池满或不可用时降级为直接创建协程执行
// @param task func() 待执行的任务
func Add(task func()) {
	if globalPool == nil {
		xcall.Go(task)
		return
	}

	if err := globalPool.AddTask(task); err != nil {
		xcall.Go(task)
		log.Warnf("add task to the task pool failed: %v", err)
		return
	}
}

// Release 释放全局任务池
func Release() {
	if globalPool != nil {
		globalPool.Release()
	}
}

// logger 任务池日志适配器，将 ants 日志桥接到 due 日志框架
type logger struct {
}

// Printf 记录格式化日志
// @param format string 日志格式
// @param args ...any 日志参数
func (l *logger) Printf(format string, args ...any) {
	log.Infof(format, args...)
}
