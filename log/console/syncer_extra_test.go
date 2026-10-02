package console_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/log/console"
)

// captureConsoleStdout captures the output written to os.Stdout.
func captureConsoleStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe failed: %v", err)
	}
	defer func() {
		os.Stdout = old
		_ = r.Close()
	}()

	os.Stdout = w

	fn()

	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout failed: %v", err)
	}

	return buf.String()
}

// newConsoleEntity builds a log entity for the console syncer tests.
func newConsoleEntity() *log.Entity {
	return &log.Entity{
		Now:     time.Now(),
		Time:    "2024/01/02 03:04:05.000000",
		Level:   log.LevelInfo,
		Message: "console message",
		Caller:  "main.go:10",
	}
}

// TestSyncer_NameAndClose verifies the syncer name and close behaviour.
func TestSyncer_NameAndClose(t *testing.T) {
	syncer := console.NewSyncer()

	if got, want := syncer.Name(), console.Name; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if err := syncer.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// TestSyncer_WriteText verifies the text output.
func TestSyncer_WriteText(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	output := captureConsoleStdout(t, func() {
		syncer := console.NewSyncer(console.WithFormat(console.FormatText))
		defer syncer.Close()

		if err := syncer.Write(newConsoleEntity()); err != nil {
			t.Errorf("Write() = %v, want nil", err)
		}
	})

	for _, want := range []string{"INFO", "console message", "main.go:10"} {
		if !strings.Contains(output, want) {
			t.Errorf("output = %q, want it to contain %q", output, want)
		}
	}
}

// TestSyncer_WriteJson verifies the JSON output.
func TestSyncer_WriteJson(t *testing.T) {
	output := captureConsoleStdout(t, func() {
		syncer := console.NewSyncer(console.WithFormat(console.FormatJson))
		defer syncer.Close()

		if err := syncer.Write(newConsoleEntity()); err != nil {
			t.Errorf("Write() = %v, want nil", err)
		}
	})

	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if decoded["msg"] != "console message" {
		t.Errorf("msg = %v, want %q", decoded["msg"], "console message")
	}
}

// TestSyncer_CheckSupportColor verifies the color support detection.
func TestSyncer_CheckSupportColor(t *testing.T) {
	t.Run("no color", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("TERM", "xterm-256color")

		output := captureConsoleStdout(t, func() {
			syncer := console.NewSyncer(console.WithFormat(console.FormatText))
			defer syncer.Close()

			_ = syncer.Write(newConsoleEntity())
		})

		if strings.Contains(output, "\x1b") {
			t.Errorf("output = %q, want no color codes", output)
		}
	})

	t.Run("dumb terminal", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "dumb")

		output := captureConsoleStdout(t, func() {
			syncer := console.NewSyncer(console.WithFormat(console.FormatText))
			defer syncer.Close()

			_ = syncer.Write(newConsoleEntity())
		})

		if strings.Contains(output, "\x1b") {
			t.Errorf("output = %q, want no color codes", output)
		}
	})

	t.Run("color terminal", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "xterm-256color")

		output := captureConsoleStdout(t, func() {
			syncer := console.NewSyncer(console.WithFormat(console.FormatText))
			defer syncer.Close()

			_ = syncer.Write(newConsoleEntity())
		})

		if !strings.Contains(output, "\x1b") {
			t.Errorf("output = %q, want color codes", output)
		}
	})
}
