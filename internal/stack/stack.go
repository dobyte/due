package stack

import (
	"fmt"
	"runtime"
	"sync"
)

type Depth int

const (
	// First captures only the first frame.
	First Depth = iota

	// Full captures the entire call stack, allocating more
	// storage for it if needed.
	Full
)

var stacks = sync.Pool{New: func() interface{} {
	return &Stack{storage: make([]uintptr, 64)}
}}

func Callers(skip int, depth Depth) *Stack {
	stack := stacks.Get().(*Stack)
	switch depth {
	case First:
		stack.pcs = stack.storage[:1]
	case Full:
		stack.pcs = stack.storage
	}

	numFrames := runtime.Callers(skip+2, stack.pcs)

	if depth == Full {
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

type Stack struct {
	pcs     []uintptr
	frames  *runtime.Frames
	storage []uintptr
}

func (st *Stack) Free() {
	st.pcs = nil
	st.frames = nil
	stacks.Put(st)
}

func (st *Stack) Next() (_ runtime.Frame, more bool) {
	return st.frames.Next()
}

func (st *Stack) Frames() []runtime.Frame {
	frames := make([]runtime.Frame, 0, len(st.pcs))
	frame, more := st.Next()
	frames = append(frames, frame)
	for more {
		// ignore runtime.main or runtime.goexit
		if frame, more = st.Next(); more {
			frames = append(frames, frame)
		}
	}

	return frames
}

func (st *Stack) String() string {
	return fmt.Sprintf("%s", st)
}

func (st *Stack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v', 's':
		for i, f := range st.Frames() {
			fmt.Fprintf(s, "%d). %s\n\t%s:%d\n",
				i+1,
				f.Function,
				f.File,
				f.Line,
			)
		}
	}
}
