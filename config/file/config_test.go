package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/file"
	"github.com/dobyte/due/v2/errors"
)

const configFile = "config.json"

// writeFile writes the given content to a file under dir, failing the test on error.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed, err: %v", err)
	}
}

func TestName(t *testing.T) {
	source := file.NewSource(file.WithPath(t.TempDir()))

	if source.Name() != file.Name {
		t.Fatalf("invalid source name, expect: %s, actual: %s", file.Name, source.Name())
	}

	if err := source.Close(); err != nil {
		t.Fatalf("close source failed, err: %v", err)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)

	source := file.NewSource(file.WithPath(dir), file.WithMode(config.ReadWrite))

	configs, err := source.Load(context.Background(), configFile)
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("invalid config count, expect: 1, actual: %d", len(configs))
	}

	cc := configs[0]
	if cc.Name != "config" {
		t.Fatalf("invalid config name, expect: config, actual: %s", cc.Name)
	}
	if cc.File != configFile {
		t.Fatalf("invalid config file, expect: %s, actual: %s", configFile, cc.File)
	}
	if cc.Format != "json" {
		t.Fatalf("invalid config format, expect: json, actual: %s", cc.Format)
	}
	if string(cc.Content) != `{"timezone":"Local"}` {
		t.Fatalf("invalid config content, actual: %s", cc.Content)
	}
}

func TestLoadDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)
	writeFile(t, dir, "gate.yaml", "name: gate\n")
	writeFile(t, dir, ".hidden.json", `{"hidden":true}`)

	source := file.NewSource(file.WithPath(dir))

	configs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("invalid config count, expect: 2, actual: %d", len(configs))
	}
}

func TestLoadMissingFile(t *testing.T) {
	source := file.NewSource(file.WithPath(t.TempDir()))

	if _, err := source.Load(context.Background(), configFile); err == nil {
		t.Fatal("expect an error for the missing config file")
	}
}

func TestLoadRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)

	source := file.NewSource(file.WithPath(dir))

	if _, err := source.Load(context.Background(), "../"+configFile); err == nil {
		t.Fatal("expect an error for the path traversal")
	}

	if _, err := source.Load(context.Background(), filepath.Join(dir, configFile)); err == nil {
		t.Fatal("expect an error for the absolute file path")
	}
}

func TestStore(t *testing.T) {
	dir := t.TempDir()

	source := file.NewSource(file.WithPath(dir), file.WithMode(config.ReadWrite))

	if err := source.Store(context.Background(), "gate.json", []byte(`{"name":"gate"}`)); err != nil {
		t.Fatalf("store config failed, err: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "gate.json"))
	if err != nil {
		t.Fatalf("read stored config failed, err: %v", err)
	}
	if string(content) != `{"name":"gate"}` {
		t.Fatalf("invalid stored content, actual: %s", content)
	}

	if err = source.Store(context.Background(), "../gate.json", []byte(`{}`)); err == nil {
		t.Fatal("expect an error for the path traversal")
	}
}

func TestStoreWithoutPermission(t *testing.T) {
	source := file.NewSource(file.WithPath(t.TempDir()), file.WithMode(config.ReadOnly))

	if err := source.Store(context.Background(), "gate.json", []byte(`{}`)); !errors.Is(err, errors.ErrNoOperationPermission) {
		t.Fatalf("expect ErrNoOperationPermission, actual: %v", err)
	}
}

func TestConfiguratorGet(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local","pid":"./run/gate.pid"}`)

	c := config.NewConfigurator(config.WithSources(file.NewSource(file.WithPath(dir))))
	defer c.Close()

	if !c.Has("config.timezone") {
		t.Fatal("expect the config key to exist")
	}
	if c.Has("config.missing") {
		t.Fatal("expect the config key not to exist")
	}

	if got := c.Get("config.timezone").String(); got != "Local" {
		t.Fatalf("invalid config value, expect: Local, actual: %s", got)
	}
	if got := c.Get("config.missing", "default").String(); got != "default" {
		t.Fatalf("invalid default config value, expect: default, actual: %s", got)
	}
}

// TestWatch rewrites the watched file until the watcher reports the change. Rewriting makes the test
// immune to the short window in which the file is truncated but not yet written.
func TestWatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)

	c := config.NewConfigurator(config.WithSources(file.NewSource(file.WithPath(dir), file.WithMode(config.ReadWrite))))
	defer c.Close()

	notified := make(chan string, 1)
	c.Watch(func(names ...string) {
		for _, name := range names {
			select {
			case notified <- name:
			default:
			}
		}
	}, "config")

	deadline := time.Now().Add(15 * time.Second)

	for {
		writeFile(t, dir, configFile, `{"timezone":"UTC"}`)

		select {
		case name := <-notified:
			if name != "config" {
				t.Fatalf("invalid notified name, expect: config, actual: %s", name)
			}

			return
		case <-time.After(250 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			t.Fatal("the watch callback is not triggered")
		}
	}
}
