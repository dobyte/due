/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 12:24 下午
 * @Desc: TODO
 */

package log

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

type entityPool struct {
	pool            sync.Pool
	stackLevel      Level
	callerFormat    CallerFormat
	timestampFormat string
	callerSkip      int
}

func newEntityPool(stackLevel Level, callerFormat CallerFormat, timestampFormat string, callerSkip int) *entityPool {
	return &entityPool{
		pool:            sync.Pool{New: func() interface{} { return &entity{} }},
		stackLevel:      stackLevel,
		callerFormat:    callerFormat,
		timestampFormat: timestampFormat,
		callerSkip:      callerSkip,
	}
}

func (p *entityPool) build(level Level, msg string) *entity {
	e := p.pool.Get().(*entity)
	e.pool = p

	switch level {
	case DebugLevel:
		e.color = gray
	case WarnLevel:
		e.color = yellow
	case ErrorLevel, FatalLevel, PanicLevel:
		e.color = red
	case InfoLevel:
		e.color = blue
	default:
		e.color = blue
	}

	e.level = level.String()[:4]
	e.time = time.Now().Format(p.timestampFormat)
	e.message = strings.TrimRight(msg, "\n")

	if p.stackLevel != defaultNoneLevel && level >= p.stackLevel {
		e.frames = GetFrames(3+e.pool.callerSkip, StacktraceFull)
		e.caller = p.framesToCaller(e.frames)
	} else {
		e.frames = GetFrames(3+e.pool.callerSkip, StacktraceFirst)
		e.caller = p.framesToCaller(e.frames)
		e.frames = nil
	}

	return e
}

func (p *entityPool) framesToCaller(frames []runtime.Frame) string {
	if len(frames) == 0 {
		return ""
	}

	file := frames[0].File
	if p.callerFormat == CallerShortPath {
		_, file = filepath.Split(file)
	}

	return fmt.Sprintf("%s:%d", file, frames[0].Line)
}

type entity struct {
	color   int
	level   string
	time    string
	caller  string
	message string
	frames  []runtime.Frame
	pool    *entityPool
}

func (e *entity) free() {
	e.color = 0
	e.level = ""
	e.time = ""
	e.caller = ""
	e.message = ""
	e.frames = nil
	e.pool.pool.Put(e)
}
