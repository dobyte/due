package log

import (
	"runtime"
	"sync"
)

type Entity struct {
	pool    *sync.Pool
	time    string
	level   Level
	message string
	caller  string
	frames  []runtime.Frame
}

// Release 释放
func (e *Entity) Release() {
	e.time = ""
	e.message = ""
	e.caller = ""
	e.frames = e.frames[:0]
	e.pool.Put(e)
}

func (e *Entity) Time() string {
	return e.time
}

func (e *Entity) Level() Level {
	return e.level
}

func (e *Entity) Message() string {
	return e.message
}

func (e *Entity) Caller() string {
	return e.caller
}

func (e *Entity) Frames() []runtime.Frame {
	return e.frames
}
