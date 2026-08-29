package internal

import (
	"bytes"
	"encoding/json"
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
		pool: &sync.Pool{New: func() any { return &buffer{bufer: bytes.NewBuffer(make([]byte, 0, 1024))} }},
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
	data, _ := json.Marshal(s)
	b.Write(data)
}
