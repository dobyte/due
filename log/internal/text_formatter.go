package internal

import (
	"bytes"
	"sync"
)

type TextFormatter struct {
	pool           *sync.Pool
	isSupportColor bool
}

func NewTextFormatter(isSupportColor ...bool) *TextFormatter {
	return &TextFormatter{
		pool:           &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(make([]byte, 0, defaultBufferSize))} }},
		isSupportColor: len(isSupportColor) > 0 && isSupportColor[0],
	}
}

func (f *TextFormatter) Format(entity *Entity) Buffer {
	b := f.pool.Get().(*buffer)
	b.pool = f.pool

	if f.isSupportColor {
		b.WriteString(entity.Level.Color())
		b.WriteString(entity.Level.Label())
		b.WriteString(reset)
	} else {
		b.WriteString(entity.Level.Label())
	}

	b.WriteRune('[')
	b.WriteString(entity.Time)
	b.WriteRune(']')

	if entity.Caller != "" {
		b.WriteRune(' ')
		b.WriteString(entity.Caller)
	}

	if entity.Message != "" {
		b.WriteRune(' ')
		b.WriteString(entity.Message)
	}

	if len(entity.Frames) > 0 {
		b.WriteString("\nStack:")
		for i, frame := range entity.Frames {
			b.WriteByte('\n')
			b.WriteInt(i + 1)
			b.WriteString(".")
			b.WriteString(frame.Function)
			b.WriteString("\n\t")
			b.WriteString(frame.File)
			b.WriteString(":")
			b.WriteInt(frame.Line)
		}
	}

	b.WriteByte('\n')

	return b
}
