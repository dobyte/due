package internal

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

type JsonFormatter struct {
	pool    *sync.Pool
	console bool
}

func NewJsonFormatter(console ...bool) *JsonFormatter {
	return &JsonFormatter{
		pool:    &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(make([]byte, 0, 1024))} }},
		console: len(console) > 0 && console[0],
	}
}

func (f *JsonFormatter) Format(entity *Entity) Buffer {
	b := f.pool.Get().(*buffer)
	b.pool = f.pool

	b.WriteString(`{"`)
	b.WriteString(fieldKeyLevel)
	b.WriteString(`":"`)

	if f.console {
		b.WriteString(entity.Level.Color())
		b.WriteString(entity.Level.Label())
		b.WriteString(reset)
	} else {
		b.WriteString(entity.Level.Label())
	}

	b.WriteString(`","`)
	b.WriteString(fieldKeyTime)
	b.WriteString(`":"`)
	b.WriteString(entity.Time)
	b.WriteString(`"`)

	if entity.Caller != "" {
		b.WriteString(`,"`)
		b.WriteString(fieldKeyFile)
		b.WriteString(`":"`)
		b.WriteString(entity.Caller)
		b.WriteString(`"`)
	}

	if entity.Message != "" {
		b.WriteString(`,"`)
		b.WriteString(fieldKeyMsg)
		b.WriteString(`":"`)
		b.WriteString(entity.Message)
		b.WriteString(`"`)
	}

	if len(entity.Frames) > 0 {
		b.WriteString(`,"`)
		b.WriteString(fieldKeyStack)
		b.WriteString(`":[`)
		for i, frame := range entity.Frames {
			if i == 0 {
				b.WriteString(`{"`)
				b.WriteString(fieldKeyStackFunc)
				b.WriteString(`":"`)
				b.WriteString(frame.Function)
				b.WriteString(`"`)
			} else {
				b.WriteString(`,{"`)
				b.WriteString(fieldKeyStackFunc)
				b.WriteString(`":"`)
				b.WriteString(frame.Function)
				b.WriteString(`"`)
			}
			b.WriteString(`,"`)
			b.WriteString(fieldKeyStackFile)
			b.WriteString(`":"`)
			b.WriteString(frame.File)
			b.WriteString(`:`)
			b.WriteString(strconv.Itoa(frame.Line))
			b.WriteString(`"`)
		}
		b.WriteString(`]`)
	}

	b.WriteString("}\n")

	return b
}
