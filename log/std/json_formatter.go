/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/6 5:01 下午
 * @Desc: TODO
 */

package std

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

type JsonFormatter struct {
	bufferPool sync.Pool
}

func newJsonFormatter() *JsonFormatter {
	return &JsonFormatter{
		bufferPool: sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
	}
}

func (f *JsonFormatter) format(e *entity, isTerminal bool) []byte {
	b := f.bufferPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		f.bufferPool.Put(b)
	}()

	fmt.Fprintf(b, `{"%s":"%s"`, fieldKeyLevel, e.level)
	fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyTime, e.time)

	if e.caller != "" {
		fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyFile, e.caller)
	}

	if e.message != "" {
		fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyMsg, e.message)
	}

	if len(e.frames) > 0 {
		fmt.Fprintf(b, `,"%s":[`, fieldKeyStack)

		for i, frame := range e.frames {
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
