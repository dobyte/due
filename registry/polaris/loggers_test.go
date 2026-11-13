package polaris

import (
	"testing"

	"github.com/polarismesh/polaris-go/api"
	plog "github.com/polarismesh/polaris-go/pkg/log"
)

// testLogger is a configurable plog.Logger used by the logger tests.
type testLogger struct {
	dir   string
	level int
}

func (l *testLogger) Tracef(string, ...interface{}) {}
func (l *testLogger) Debugf(string, ...interface{}) {}
func (l *testLogger) Infof(string, ...interface{})  {}
func (l *testLogger) Warnf(string, ...interface{})  {}
func (l *testLogger) Errorf(string, ...interface{}) {}
func (l *testLogger) Fatalf(string, ...interface{}) {}
func (l *testLogger) IsLevelEnabled(level int) bool { return level >= l.level }
func (l *testLogger) SetLogLevel(level int) error   { l.level = level; return nil }
func (l *testLogger) GetLogDir() string             { return l.dir }

func TestLoggersStillDefault(t *testing.T) {
	if !loggersStillDefault() {
		t.Fatal("loggersStillDefault() = false, want true for the default SDK loggers")
	}

	base := api.GetBaseLogger()
	defer api.SetBaseLogger(base)

	api.SetBaseLogger(&testLogger{dir: plog.DefaultLogRotationRootDir, level: plog.WarnLog})

	if loggersStillDefault() {
		t.Fatal("loggersStillDefault() = true, want false for a customized log level")
	}

	api.SetBaseLogger(&testLogger{dir: "custom/log", level: plog.DefaultLogLevel})

	if loggersStillDefault() {
		t.Fatal("loggersStillDefault() = true, want false for a customized log directory")
	}
}
