package log

import (
	"log"
)

var formatters = make(map[string]Formatter)

type Formatter interface {
	// Name 名称
	Name() string
	// Format 格式化
	Format(entity *Entity, isConsole ...bool) Buffer
}

func init() {
	RegisterFormatter(newTextFormatter())
	RegisterFormatter(newJsonFormatter())
}

// RegisterFormatter 注册编解码器
func RegisterFormatter(formatter Formatter) {
	if formatter == nil {
		log.Fatal("can't register a invalid formatter")
	}

	name := formatter.Name()

	if name == "" {
		log.Fatal("can't register a codec without name")
	}

	if _, ok := formatters[name]; ok {
		log.Printf("the old %s formatter will be overwritten", name)
	}

	formatters[name] = formatter
}
