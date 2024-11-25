/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 12:30 下午
 * @Desc: TODO
 */

package log

import (
	"bytes"
	"strconv"
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
		b.WriteString("\x1b[" + strconv.Itoa(e.Color) + "m" + level + "\x1b[0m[" + e.Time + "]")
	} else {
		b.WriteString(level + "[" + e.Time + "]")
	}

	if e.Caller != "" {
		b.WriteString(" " + e.Caller)
	}

	if e.Message != "" {
		b.WriteString(" " + e.Message)
	}

	if len(e.Frames) > 0 {
		b.WriteString("\nStack:")
		for i, frame := range e.Frames {
			b.WriteString("\n" + strconv.Itoa(i+1) + "." + frame.Function + "\n\t" + frame.File + ":" + strconv.Itoa(frame.Line))
		}
	}

	b.WriteByte('\n')

	return b.Bytes()
}
