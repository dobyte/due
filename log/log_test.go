package log_test

import (
	"github.com/symsimmy/due/log"
	"testing"
	"time"
)

var logger = log.NewLogger()

func TestDefaultLogger(t *testing.T) {
	timeTick := time.Tick(1 * time.Second)
	timeAfter := time.After(180 * time.Second)
	for {
		select {
		case <-timeTick:
			logger.Info("info")
		case <-timeAfter:
			return
		}
	}
}
