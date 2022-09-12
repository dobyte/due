/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 12:30 下午
 * @Desc: TODO
 */

package log

import (
	"bytes"
	"fmt"
	"sync"
)

type textFormatter struct {
	bufferPool sync.Pool
}

func newTextFormatter() *textFormatter {
	return &textFormatter{
		bufferPool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
	}
}

func (f *textFormatter) format(e *Entity, isTerminal bool) []byte {
	b := f.bufferPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		f.bufferPool.Put(b)
	}()

	level := e.Level.String()[:4]

	if isTerminal {
		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s]", e.Color, level, e.Time)
	} else {
		fmt.Fprintf(b, "%s[%s]", level, e.Time)
	}

	if e.Caller != "" {
		fmt.Fprint(b, " "+e.Caller)
	}

	if e.Message != "" {
		fmt.Fprint(b, " "+e.Message)
	}

	if len(e.Frames) > 0 {
		fmt.Fprint(b, "\n")
		fmt.Fprint(b, "Stack:")
		for i, frame := range e.Frames {
			fmt.Fprintf(b, "\n%d.%s\n", i+1, frame.Function)
			fmt.Fprintf(b, "\t%s:%d", frame.File, frame.Line)
		}
	}

	fmt.Fprintf(b, "\n")

	return b.Bytes()
}
