package xos_test

import (
	"os"
	"path/filepath"
	"testing"

	xos "github.com/dobyte/due/v2/utils/xos"
)

func TestStat(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a.txt")
		if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}

		info, err := xos.Stat(path)
		if err != nil {
			t.Fatalf("stat existing file failed: %v", err)
		}
		if info.Name() != "a.txt" {
			t.Fatalf("unexpected name: %s", info.Name())
		}
		if !info.IsFile() {
			t.Fatalf("expected a regular file")
		}
	})

	t.Run("not existing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-exist.txt")
		if _, err := xos.Stat(path); err == nil {
			t.Fatalf("expected an error for non-existing path")
		}
	})
}

func TestIsDir(t *testing.T) {
	dir := t.TempDir()

	t.Run("directory", func(t *testing.T) {
		if !xos.IsDir(dir) {
			t.Fatalf("expected %s to be a directory", dir)
		}
	})

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(dir, "a.txt")
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
		if xos.IsDir(path) {
			t.Fatalf("expected %s not to be a directory", path)
		}
	})

	t.Run("not existing", func(t *testing.T) {
		if xos.IsDir(filepath.Join(dir, "not-exist")) {
			t.Fatalf("expected false for non-existing path")
		}
	})
}

func TestIsFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(dir, "a.txt")
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
		if !xos.IsFile(path) {
			t.Fatalf("expected %s to be a file", path)
		}
	})

	t.Run("directory", func(t *testing.T) {
		if xos.IsFile(dir) {
			t.Fatalf("expected %s not to be a file", dir)
		}
	})

	t.Run("not existing", func(t *testing.T) {
		if xos.IsFile(filepath.Join(dir, "not-exist")) {
			t.Fatalf("expected false for non-existing path")
		}
	})
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		dir, file string
		base, ext string
	}{
		{"with ext", "run/test.txt", "run/", "test.txt", "test", "txt"},
		{"no ext", "Makefile", "", "Makefile", "Makefile", ""},
		{"hidden file", ".gitignore", "", ".gitignore", ".gitignore", ""},
		{"multi ext", "archive.tar.gz", "", "archive.tar.gz", "archive.tar", "gz"},
		{"dir only", "run/", "run/", "", "", ""},
		{"empty", "", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, file, base, ext := xos.Split(tt.path)
			if dir != tt.dir || file != tt.file || base != tt.base || ext != tt.ext {
				t.Fatalf("Split(%q) = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
					tt.path, dir, file, base, ext, tt.dir, tt.file, tt.base, tt.ext)
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	t.Run("existing dir", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a.txt")
		if err := xos.WriteFile(path, []byte("hello")); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
		assertFileContent(t, path, "hello")
	})

	t.Run("create dir", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "dir", "a.txt")
		if err := xos.WriteFile(path, []byte("world")); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
		assertFileContent(t, path, "world")
	})

	t.Run("mkdirall error", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, nil, 0644); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(blocker, "sub", "a.txt")
		if err := xos.WriteFile(path, nil); err == nil {
			t.Fatalf("expected an error when parent path is a file")
		}
	})

	t.Run("write error", func(t *testing.T) {
		dir := t.TempDir()
		if err := xos.WriteFile(dir, nil); err == nil {
			t.Fatalf("expected an error when target is a directory")
		}
	})
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(data) != want {
		t.Fatalf("unexpected content: %q, want %q", data, want)
	}
}
