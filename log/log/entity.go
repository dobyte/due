package log

import (
	"runtime"
	"sync"
	"time"
)

type entity struct {
	pool    *sync.Pool
	level   Level
	time    time.Time
	caller  string
	message string
	frames  []runtime.Frame
}

func (e *entity) free() {
	e.frames = nil
	e.pool.Put(e)
}
