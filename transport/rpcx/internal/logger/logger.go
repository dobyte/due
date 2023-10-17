package logger

import (
	"github.com/dobyte/due/v2/log"
	rpcxlog "github.com/smallnest/rpcx/log"
	"sync"
)

var once sync.Once

func InitLogger() {
	once.Do(func() {
		rpcxlog.SetLogger(log.GetLogger())
	})
}
