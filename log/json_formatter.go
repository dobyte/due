/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 5:01 下午
 * @Desc: TODO
 */

package log

import (
	"bytes"
	"fmt"
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

	fmt.Fprintf(b, `{"%s":"%s"`, fieldKeyLevel, e.Level.String()[:4])
	fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyTime, e.Time)

	if e.Caller != "" {
		fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyFile, e.Caller)
	}

	if e.Message != "" {
		fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyMsg, e.Message)
	}

	if len(e.Frames) > 0 {
		fmt.Fprintf(b, `,"%s":[`, fieldKeyStack)

		for i, frame := range e.Frames {
			if i == 0 {
				fmt.Fprintf(b, `{"%s":"%s"`, fieldKeyStackFunc, frame.Function)
			} else {
				fmt.Fprintf(b, `,{"%s":"%s"`, fieldKeyStackFunc, frame.Function)
			}
			fmt.Fprintf(b, `,"%s":"%s:%d"}`, fieldKeyStackFile, frame.File, frame.Line)
		}

		fmt.Fprint(b, "]")
	}
	fmt.Fprint(b, "}\n")

	return b.Bytes()
}
