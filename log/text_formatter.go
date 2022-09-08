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

func (f *textFormatter) format(e *entity, isTerminal bool) []byte {
	b := f.bufferPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		f.bufferPool.Put(b)
	}()

	if isTerminal {
		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s]", e.color, e.level, e.time)
	} else {
		fmt.Fprintf(b, "%s[%s]", e.level, e.time)
	}

	if e.caller != "" {
		fmt.Fprint(b, " "+e.caller)
	}

	if e.message != "" {
		fmt.Fprint(b, " "+e.message)
	}

	if len(e.frames) > 0 {
		fmt.Fprint(b, "\n")
		fmt.Fprint(b, "Stack:")
		for i, frame := range e.frames {
			fmt.Fprintf(b, "\n%d.%s\n", i+1, frame.Function)
			fmt.Fprintf(b, "\t%s:%d", frame.File, frame.Line)
		}
	}

	fmt.Fprintf(b, "\n")

	return b.Bytes()
}
