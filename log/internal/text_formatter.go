package internal

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
)

type TextFormatter struct {
	pool    *sync.Pool
	console bool
}

func NewTextFormatter(console ...bool) *TextFormatter {
	return &TextFormatter{
		pool:    &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(nil)} }},
		console: len(console) > 0 && console[0],
	}
}

func (f *TextFormatter) Format(entity *Entity) Buffer {
	b := f.pool.Get().(*buffer)
	b.pool = f.pool

	level := strings.ToUpper(string(entity.Level[:4]))

	if f.console {
		b.WriteString("\x1b[" + strconv.Itoa(entity.Level.Color()) + "m" + level + "\x1b[0m[" + entity.Time + "]")
	} else {
		b.WriteString(level + "[" + entity.Time + "]")
	}

	if entity.Caller != "" {
		b.WriteString(" " + entity.Caller)
	}

	if entity.Message != "" {
		b.WriteString(" " + entity.Message)
	}

	if len(entity.Frames) > 0 {
		b.WriteString("\nStack:")
		for i, frame := range entity.Frames {
			b.WriteString("\n" + strconv.Itoa(i+1) + "." + frame.Function + "\n\t" + frame.File + ":" + strconv.Itoa(frame.Line))
		}
	}

	b.WriteByte('\n')

	return b
}
