package log

import (
	"log"

	"github.com/dobyte/due/v2/log/console"
	"github.com/dobyte/due/v2/log/file"
)

var syncers = make(map[string]Syncer)

type Syncer interface {
	// Name 同步器名称
	Name() string
	// Write 写入日志
	Write(entity *Entity) error
	// Close 关闭同步器
	Close() error
}

func init() {
	RegisterSyncer(file.NewSyncer())
	RegisterSyncer(console.NewSyncer())
}

// RegisterSyncer 注册同步器
func RegisterSyncer(syncer Syncer) {
	if syncer == nil {
		log.Fatal("can't register a invalid syncer")
	}

	name := syncer.Name()

	if name == "" {
		log.Fatal("can't register a syncer without name")
	}

	if _, ok := syncers[name]; ok {
		log.Printf("the old %s syncer will be overwritten", name)
	}

	syncers[name] = syncer
}
