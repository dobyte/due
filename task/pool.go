// Package task provides a global task scheduling pool and a task group.
//
// It is built on the ants goroutine pool and supports concurrency control, error aggregation and
// graceful degradation.
package task

import (
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/panjf2000/ants/v2"
)

// Pool is a task pool.
type Pool interface {
	// AddTask adds task to the pool and returns any error.
	AddTask(task func()) error
	// Release releases the task pool.
	Release()
}

// globalPool is the global task pool.
var globalPool Pool

// init initializes the global task pool.
func init() {
	SetPool(NewPool())
}

// defaultPool is the default task pool implementation based on ants.
type defaultPool struct {
	pool *ants.Pool
}

// NewPool returns a new task pool.
//
// It creates an ants goroutine pool from the configured options and supports features such as the
// pool size, non-blocking mode and disabled purging.
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

// AddTask adds task to the pool and returns any error.
func (p *defaultPool) AddTask(task func()) error {
	return p.pool.Submit(task)
}

// Release releases the task pool.
func (p *defaultPool) Release() {
	p.pool.Release()
}

// SetPool sets the global task pool, releasing the old pool first.
func SetPool(pool Pool) {
	if globalPool != nil {
		globalPool.Release()
	}
	globalPool = pool
}

// GetPool returns the global task pool.
func GetPool() Pool {
	return globalPool
}

// AddTask adds task to the global task pool.
//
// Deprecated: As of due v2.6.0+, this function simply calls [Add].
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

// Add runs task.
//
// The task is submitted to the global task pool first; it degrades to running in a directly
// created goroutine when the pool is full or unavailable.
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

// Release releases the global task pool.
func Release() {
	if globalPool != nil {
		globalPool.Release()
	}
}

// logger is a task pool log adapter that bridges ants logs to the due log framework.
type logger struct {
}

// Printf logs a formatted message.
func (l *logger) Printf(format string, args ...any) {
	log.Infof(format, args...)
}
