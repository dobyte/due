/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 12:24 下午
 * @Desc: TODO
 */

package log

import (
	"fmt"
	"github.com/symsimmy/due/internal/stack"
	"github.com/symsimmy/due/utils/xtime"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

type EntityPool struct {
	pool   sync.Pool
	logger *defaultLogger
}

func newEntityPool(logger *defaultLogger) *EntityPool {
	return &EntityPool{
		pool:   sync.Pool{New: func() interface{} { return &Entity{} }},
		logger: logger,
	}
}

func (p *EntityPool) build(level Level, a ...interface{}) *Entity {
	e := p.pool.Get().(*Entity)
	e.pool = p

	switch level {
	case DebugLevel:
		e.Color = gray
	case WarnLevel:
		e.Color = yellow
	case ErrorLevel, FatalLevel, PanicLevel:
		e.Color = red
	case InfoLevel:
		e.Color = blue
	default:
		e.Color = blue
	}

	var msg string
	if c := len(a); c > 0 {
		msg = fmt.Sprintf(strings.TrimSuffix(strings.Repeat("%v ", c), " "), a...)
	}

	e.Level = level
	e.Time = xtime.Now().Format(p.logger.opts.timeFormat)
	e.Message = strings.TrimSuffix(msg, "\n")

	if p.logger.opts.stackLevel != 0 && level >= p.logger.opts.stackLevel {
		st := stack.Callers(3+p.logger.opts.callerSkip, stack.Full)
		defer st.Free()
		e.Frames = st.Frames()
		e.Caller = p.framesToCaller(e.Frames)
	} else {
		st := stack.Callers(3+p.logger.opts.callerSkip, stack.First)
		defer st.Free()
		e.Frames = st.Frames()
		e.Caller = p.framesToCaller(e.Frames)
		e.Frames = nil
	}

	return e
}

func (p *EntityPool) framesToCaller(frames []runtime.Frame) string {
	if len(frames) == 0 {
		return ""
	}

	file := frames[0].File
	if !p.logger.opts.callerFullPath {
		_, file = filepath.Split(file)
	}

	return fmt.Sprintf("%s:%d", file, frames[0].Line)
}

type Entity struct {
	Color   int
	Level   Level
	Time    string
	Caller  string
	Message string
	Frames  []runtime.Frame
	pool    *EntityPool
}

func (e *Entity) Free() {
	e.Color = 0
	e.Level = 0
	e.Time = ""
	e.Caller = ""
	e.Message = ""
	e.Frames = nil
	e.pool.pool.Put(e)
}

func (e *Entity) Log() {
	defer e.Free()

	if e.Level < e.pool.logger.opts.level {
		return
	}

	buffers := make(map[bool][]byte, 2)
	for _, s := range e.pool.logger.syncers {
		if !s.enabler(e.Level) {
			continue
		}
		b, ok := buffers[s.terminal]
		if !ok {
			b = e.pool.logger.formatter.format(e, s.terminal)
			buffers[s.terminal] = b
		}
		s.writer.Write(b)
	}
}
