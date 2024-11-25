/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 5:01 下午
 * @Desc: TODO
 */

package log

import (
	"bytes"
	"strconv"
	"sync"
)

const (
	fieldKeyLevel     = "level"
	fieldKeyTime      = "time"
	fieldKeyFile      = "file"
	fieldKeyMsg       = "msg"
	fieldKeyStack     = "stack"
	fieldKeyStackFunc = "func"
	fieldKeyStackFile = "file"
)

type jsonFormatter struct {
	bufferPool sync.Pool
}

func newJsonFormatter() *jsonFormatter {
	return &jsonFormatter{
		bufferPool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
	}
}

func (f *jsonFormatter) format(e *Entity, isTerminal bool) []byte {
	b := f.bufferPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		f.bufferPool.Put(b)
	}()

	level := e.Level.String()[:4]

	b.WriteString(`{"` + fieldKeyLevel + `":"` + level + `","` + fieldKeyTime + `":"` + e.Time + `"`)

	if e.Caller != "" {
		b.WriteString(`,"` + fieldKeyFile + `":"` + e.Caller + `"`)
	}

	if e.Message != "" {
		b.WriteString(`,"` + fieldKeyMsg + `":"` + e.Message + `"`)
	}

	if len(e.Frames) > 0 {
		b.WriteString(`,"` + fieldKeyStack + `":[`)

		for i, frame := range e.Frames {
			if i == 0 {
				b.WriteString(`{"` + fieldKeyStackFunc + `":"` + frame.Function + `"`)
			} else {
				b.WriteString(`,{"` + fieldKeyStackFunc + `":"` + frame.Function + `"`)
			}
			b.WriteString(`,"` + fieldKeyStackFile + `":"` + frame.File + `:` + strconv.Itoa(frame.Line) + `"}`)
		}
		b.WriteString(`]`)
	}

	b.WriteString("}\n")

	return b.Bytes()
}
