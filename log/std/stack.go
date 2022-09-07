/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 10:33 上午
 * @Desc: TODO
 */

package std

import (
	"runtime"
	"sync"
)

type StacktraceDepth int

const (
	// stacktraceFirst captures only the first frame.
	StacktraceFirst StacktraceDepth = iota

	// stacktraceFull captures the entire call stack, allocating more
	// storage for it if needed.
	StacktraceFull
)

type stacktrace struct {
	pcs     []uintptr
	frames  *runtime.Frames
	storage []uintptr
}

var stackPool = sync.Pool{New: func() interface{} { return &stacktrace{storage: make([]uintptr, 64)} }}

func GetStacktrace(skip int, depth StacktraceDepth) *stacktrace {
	stack := stackPool.Get().(*stacktrace)

	switch depth {
	case StacktraceFirst:
		stack.pcs = stack.storage[:1]
	case StacktraceFull:
		stack.pcs = stack.storage
	}

	numFrames := runtime.Callers(
		skip+2,
		stack.pcs,
	)

	if depth == StacktraceFull {
		pcs := stack.pcs
		for numFrames == len(pcs) {
			pcs = make([]uintptr, len(pcs)*2)
			numFrames = runtime.Callers(skip+2, pcs)
		}

		stack.pcs = pcs[:numFrames]
		stack.storage = pcs
	} else {
		stack.pcs = stack.pcs[:numFrames]
	}

	stack.frames = runtime.CallersFrames(stack.pcs)

	return stack
}

func (s *stacktrace) Free() {
	s.pcs = nil
	s.frames = nil
	stackPool.Put(s)
}

func (st *stacktrace) Next() (_ runtime.Frame, more bool) {
	return st.frames.Next()
}

func GetFrames(skip int, depth StacktraceDepth) []runtime.Frame {
	stack := GetStacktrace(skip+1, depth)
	defer stack.Free()

	frames := make([]runtime.Frame, 0, len(stack.pcs))
	frame, more := stack.Next()
	frames = append(frames, frame)
	for more {
		// ignore runtime.main or runtime.goexit
		if frame, more = stack.Next(); more {
			frames = append(frames, frame)
		}
	}

	return frames
}
