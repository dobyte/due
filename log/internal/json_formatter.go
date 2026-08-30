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
	pool *sync.Pool
}

func NewJsonFormatter() *JsonFormatter {
	return &JsonFormatter{
		pool: &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(make([]byte, 0, defaultBufferSize))} }},
	}
}

func (f *JsonFormatter) Format(entity *Entity) Buffer {
	b := f.pool.Get().(*buffer)
	b.pool = f.pool

	b.WriteString(`{"`)
	b.WriteString(fieldKeyLevel)
	b.WriteString(`":`)
	writeJSONString(b, entity.Level.Label())

	b.WriteString(`,"`)
	b.WriteString(fieldKeyTime)
	b.WriteString(`":`)
	writeJSONString(b, entity.Time)

	if entity.Caller != "" {
		b.WriteString(`,"`)
		b.WriteString(fieldKeyFile)
		b.WriteString(`":`)
		writeJSONString(b, entity.Caller)
	}

	if entity.Message != "" {
		b.WriteString(`,"`)
		b.WriteString(fieldKeyMsg)
		b.WriteString(`":`)
		writeJSONString(b, entity.Message)
	}

	if len(entity.Frames) > 0 {
		b.WriteString(`,"`)
		b.WriteString(fieldKeyStack)
		b.WriteString(`":[`)
		for i, frame := range entity.Frames {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"`)
			b.WriteString(fieldKeyStackFunc)
			b.WriteString(`":`)
			writeJSONString(b, frame.Function)
			b.WriteString(`,"`)
			b.WriteString(fieldKeyStackFile)
			b.WriteString(`":`)
			writeJSONString(b, frame.File+":"+strconv.Itoa(frame.Line))
			b.WriteString(`}`)
		}
		b.WriteString(`]`)
	}

	b.WriteString("}\n")

	return b
}

func writeJSONString(b *buffer, s string) {
	const hex = "0123456789abcdef"

	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '<':
			b.WriteString(`\u003c`)
		case '>':
			b.WriteString(`\u003e`)
		case '&':
			b.WriteString(`\u0026`)
		default:
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0x0f])
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
}
