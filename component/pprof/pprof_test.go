package pprof

import (
	"testing"

	"github.com/dobyte/due/v2/log"
)

// panickingLogger is a log.Logger whose fatal methods panic instead of terminating the process.
// It lets the tests cover the fatal branches of the component.
type panickingLogger struct {
	log.Logger
}

// Fatal panics with a fixed message.
func (l *panickingLogger) Fatal(a ...any) { panic("fatal") }

// Fatalf panics with a fixed message.
func (l *panickingLogger) Fatalf(format string, a ...any) { panic("fatal") }

// Close reports success without doing anything.
func (l *panickingLogger) Close() error { return nil }

// TestNewPProf verifies the default and explicit listen addresses.
func TestNewPProf(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{name: "default addr", opts: nil, want: defaultAddr},
		{name: "custom addr", opts: []Option{WithAddr("127.0.0.1:6060")}, want: "127.0.0.1:6060"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPProf(tt.opts...)
			if got := p.opts.addr; got != tt.want {
				t.Errorf("invalid addr, expect: %s, actual: %s", tt.want, got)
			}
			if got := p.Name(); got != "pprof" {
				t.Errorf("invalid name, expect: pprof, actual: %s", got)
			}
		})
	}
}

// TestPProfStartInvalidAddr verifies that an invalid listen address aborts the startup.
func TestPProfStartInvalidAddr(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect the startup to fail with an invalid addr")
		}
	}()

	NewPProf(WithAddr("invalid-addr")).Start()
}

// TestPProfStart verifies that the component starts an HTTP server on the configured address.
func TestPProfStart(t *testing.T) {
	p := NewPProf(WithAddr("127.0.0.1:0"))

	// The pprof component has no shutdown hook, so the server keeps running until the test
	// process exits; only the startup path is exercised here.
	p.Start()
}
