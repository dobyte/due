package stat_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/stat"
)

func TestStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	fi, err := stat.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if name := fi.Name(); name != "test.txt" {
		t.Errorf("unexpected name: %s", name)
	}
	if size := fi.Size(); size != 5 {
		t.Errorf("unexpected size: %d", size)
	}
	if fi.IsDir() {
		t.Error("expected a regular file, got a directory")
	}
	if !fi.IsFile() {
		t.Error("expected a regular file")
	}
	if fi.Sys() == nil {
		t.Error("expected system data")
	}
	if mt := fi.ModifyTime(); mt.IsZero() || time.Since(mt).Abs() > time.Minute {
		t.Errorf("unexpected modify time: %s", mt)
	}
	if ct := fi.CreateTime(); ct.IsZero() {
		t.Error("expected a non-zero create time")
	}
}

func TestStatDir(t *testing.T) {
	fi, err := stat.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if !fi.IsDir() {
		t.Error("expected a directory")
	}
	if fi.IsFile() {
		t.Error("expected a directory, got a regular file")
	}
}

func TestStatNotExist(t *testing.T) {
	if _, err := stat.Stat(filepath.Join(t.TempDir(), "not-exist")); err == nil {
		t.Error("expected an error for a nonexistent file")
	}
}
