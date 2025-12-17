package log_test

import (
	"testing"

	"github.com/dobyte/due/v2/log"
)

func TestLog(t *testing.T) {
	logger := log.NewLogger()

	logger.Debug("welcome to due-framework")
	logger.Info("welcome to due-framework")
	logger.Warn("welcome to due-framework")
	logger.Error("welcome to due-framework")
}

func TestLogger(t *testing.T) {
	log.SetLogger(log.NewLogger(log.WithLevel(log.LevelDebug)))

	log.Debug("welcome to due-framework")
	log.Info("welcome to due-framework")
	log.Warn("welcome to due-framework")
	log.Error("welcome to due-framework")
}
