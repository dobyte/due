package log

import (
	"bytes"
	"strconv"
	"strings"
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
	pool *sync.Pool
}

func newJsonFormatter() Formatter {
	return &jsonFormatter{
		pool: &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(nil)} }},
	}
}

func (f *jsonFormatter) Name() string {
	return "json"
}

func (f *jsonFormatter) Format(entity *Entity, isConsole ...bool) Buffer {
	b := f.pool.Get().(*buffer)
	b.pool = f.pool

	level := strings.ToUpper(string(entity.Level()[:4]))

	if len(isConsole) > 0 && isConsole[0] {
		b.WriteString(`{"` + fieldKeyLevel + `":"` + "\x1b[" + strconv.Itoa(entity.Level().Color()) + "m" + level + "\x1b[0m" + `","` + fieldKeyTime + `":"` + entity.Time() + `"`)
	} else {
		b.WriteString(`{"` + fieldKeyLevel + `":"` + level + `","` + fieldKeyTime + `":"` + entity.Time() + `"`)
	}

	if entity.Caller() != "" {
		b.WriteString(`,"` + fieldKeyFile + `":"` + entity.Caller() + `"`)
	}

	if entity.Message() != "" {
		b.WriteString(`,"` + fieldKeyMsg + `":"` + entity.Message() + `"`)
	}

	if frames := entity.Frames(); len(frames) > 0 {
		b.WriteString(`,"` + fieldKeyStack + `":[`)
		for i, frame := range frames {
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
