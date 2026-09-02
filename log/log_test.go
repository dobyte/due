package log_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/log/console"
)

func TestLog(t *testing.T) {
	logger := log.NewLogger(log.WithSyncers(console.NewSyncer(console.WithFormat(console.FormatJson))))

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

// TestConsoleSyncerJSON 验证 console 同步器 JSON 输出合法且字段正确
func TestConsoleSyncerJSON(t *testing.T) {
	entity := &log.Entity{
		Time:    "2026/08/29 12:00:00.000000",
		Level:   log.LevelInfo,
		Message: "hello \"world\" \\ path\nwith newline",
		Caller:  "main.go:10",
	}

	output := captureStdout(t, func() {
		syncer := console.NewSyncer(console.WithFormat(console.FormatJson))
		defer syncer.Close()

		if err := syncer.Write(entity); err != nil {
			t.Errorf("write json log failed: %v", err)
		}
	})

	if strings.Contains(output, "\x1b") {
		t.Errorf("json output should not contain ANSI color codes: %q", output)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}

	if got["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", got["level"])
	}
	if got["time"] != entity.Time {
		t.Errorf("time = %v, want %v", got["time"], entity.Time)
	}
	if got["msg"] != entity.Message {
		t.Errorf("msg = %v, want %v", got["msg"], entity.Message)
	}
	if got["file"] != entity.Caller {
		t.Errorf("file = %v, want %v", got["file"], entity.Caller)
	}
}

// TestConsoleSyncerText 验证 console 同步器文本输出包含级别、消息与调用位置
func TestConsoleSyncerText(t *testing.T) {
	entity := &log.Entity{
		Time:    "2026/08/29 12:00:00.000000",
		Level:   log.LevelWarn,
		Message: "something went wrong",
		Caller:  "main.go:20",
	}

	output := captureStdout(t, func() {
		syncer := console.NewSyncer(console.WithFormat(console.FormatText))
		defer syncer.Close()

		if err := syncer.Write(entity); err != nil {
			t.Errorf("write text log failed: %v", err)
		}
	})

	if !strings.Contains(output, "WARN") {
		t.Errorf("output should contain level label WARN: %q", output)
	}
	if !strings.Contains(output, "something went wrong") {
		t.Errorf("output should contain message: %q", output)
	}
	if !strings.Contains(output, "main.go:20") {
		t.Errorf("output should contain caller: %q", output)
	}
}

// TestConsoleSyncerTextNoColor 验证设置 NO_COLOR 后不输出 ANSI 颜色码
func TestConsoleSyncerTextNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	entity := &log.Entity{
		Time:    "2026/08/29 12:00:00.000000",
		Level:   log.LevelWarn,
		Message: "something went wrong",
		Caller:  "main.go:20",
	}

	output := captureStdout(t, func() {
		syncer := console.NewSyncer(console.WithFormat(console.FormatText))
		defer syncer.Close()

		if err := syncer.Write(entity); err != nil {
			t.Errorf("write text log failed: %v", err)
		}
	})

	if strings.Contains(output, "\x1b") {
		t.Errorf("output should not contain ANSI color codes when NO_COLOR is set: %q", output)
	}
}

// captureStdout 捕获 os.Stdout 输出
func captureStdout(t *testing.T, fn func()) string {
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
