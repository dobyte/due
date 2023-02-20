package task

import (
	"github.com/dobyte/due/log"
	"github.com/panjf2000/ants/v2"
)

type Pool interface {
	// AddTask 添加任务
	AddTask(task func()) error
	// Release 释放任务
	Release()
}

var globalPool Pool

func init() {
	SetPool(NewPool())
}

type defaultPool struct {
	pool *ants.Pool
}

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
func (p *defaultPool) AddTask(task func()) error {
	return p.pool.Submit(task)
}

// Release 释放任务
func (p *defaultPool) Release() {
	p.pool.Release()
}

// SetPool 设置任务池
func SetPool(pool Pool) {
	if globalPool != nil {
		globalPool.Release()
	}
	globalPool = pool
}

// GetPool 获取任务池
func GetPool() Pool {
	return globalPool
}

// AddTask 添加任务
func AddTask(task func()) error {
	if globalPool != nil {
		return globalPool.AddTask(task)
	}

	log.Warn("the task pool component is not injected, and the task will be executed in a blocking manner.")
	task()
	return nil
}

// Release 释放任务
func Release() {
	if globalPool != nil {
		globalPool.Release()
	}
}

type logger struct {
}

func (l *logger) Printf(format string, args ...interface{}) {
	log.Infof(format, args...)
}
