package log

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
)

type textFormatter struct {
	pool *sync.Pool
}

func newTextFormatter() Formatter {
	return &textFormatter{
		pool: &sync.Pool{New: func() any { return &Buffer{bufer: bytes.NewBuffer(nil)} }},
	}
}

func (f *textFormatter) Name() string {
	return "text"
}

func (f *textFormatter) Format(entity *Entity, isConsole ...bool) *Buffer {
	b := f.pool.Get().(*Buffer)
	b.pool = f.pool

	level := strings.ToUpper(string(entity.Level()[:4]))

	if len(isConsole) > 0 && isConsole[0] {
		b.WriteString("\x1b[" + strconv.Itoa(entity.Level().Color()) + "m" + level + "\x1b[0m[" + entity.Time() + "]")
	} else {
		b.WriteString(level + "[" + entity.Time() + "]")
	}

	if entity.Caller() != "" {
		b.WriteString(" " + entity.Caller())
	}

	if entity.Message() != "" {
		b.WriteString(" " + entity.Message())
	}

	if frames := entity.Frames(); len(frames) > 0 {
		b.WriteString("\nStack:")
		for i, frame := range frames {
			b.WriteString("\n" + strconv.Itoa(i+1) + "." + frame.Function + "\n\t" + frame.File + ":" + strconv.Itoa(frame.Line))
		}
	}

	b.WriteByte('\n')

	return b
}
