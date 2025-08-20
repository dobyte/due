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
		pool:    &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(nil)} }},
		console: len(console) > 0 && console[0],
	}
}

func (f *JsonFormatter) Format(entity *Entity) Buffer {
	b := f.pool.Get().(*buffer)
	b.pool = f.pool

	if f.console {
		b.WriteString(`{"` + fieldKeyLevel + `":"` + "\x1b[" + entity.Level.Color() + "m" + entity.Level.Label() + "\x1b[0m" + `","` + fieldKeyTime + `":"` + entity.Time + `"`)
	} else {
		b.WriteString(`{"` + fieldKeyLevel + `":"` + entity.Level.Label() + `","` + fieldKeyTime + `":"` + entity.Time + `"`)
	}

	if entity.Caller != "" {
		b.WriteString(`,"` + fieldKeyFile + `":"` + entity.Caller + `"`)
	}

	if entity.Message != "" {
		b.WriteString(`,"` + fieldKeyMsg + `":"` + entity.Message + `"`)
	}

	if len(entity.Frames) > 0 {
		b.WriteString(`,"` + fieldKeyStack + `":[`)
		for i, frame := range entity.Frames {
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

	return b
}
