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

// TestLoadIgnoresHiddenDirectory verifies that the files of a dot directory are not loaded.
func TestLoadIgnoresHiddenDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)

	hidden := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatalf("create directory failed, err: %v", err)
	}
	writeFile(t, hidden, "secret.json", `{"secret":true}`)

	source := file.NewSource(file.WithPath(dir))

	configs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("invalid config count, expect: 1, actual: %d", len(configs))
	}
	if configs[0].File != configFile {
		t.Fatalf("invalid config file, expect: %s, actual: %s", configFile, configs[0].File)
	}
}

// TestLoadDotNamedRoot verifies that a dot-named source directory itself is still loaded.
func TestLoadDotNamedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".config")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create directory failed, err: %v", err)
	}
	writeFile(t, root, configFile, `{"timezone":"Local"}`)

	source := file.NewSource(file.WithPath(root))

	configs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("invalid config count, expect: 1, actual: %d", len(configs))
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

// TestStoreTriggersWatch verifies that a config stored by the file source is written atomically and
// reported by the watcher.
func TestStoreTriggersWatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)

	c := config.NewConfigurator(config.WithSources(file.NewSource(
		file.WithPath(dir),
		file.WithMode(config.ReadWrite),
		file.WithFiles(configFile),
	)))
	defer c.Close()

	notified := make(chan string, 2)
	c.Watch(func(names ...string) {
		for _, name := range names {
			select {
			case notified <- name:
			default:
			}
		}
	}, "config")

	if err := c.Store(context.Background(), file.Name, configFile, map[string]any{"timezone": "UTC"}, true); err != nil {
		t.Fatalf("store config failed, err: %v", err)
	}

	deadline := time.Now().Add(15 * time.Second)

	for {
		select {
		case name := <-notified:
			if name != "config" {
				t.Fatalf("invalid notified name, expect: config, actual: %s", name)
			}

			if got := c.Get("config.timezone").String(); got != "UTC" {
				t.Fatalf("invalid value, expect: UTC, actual: %s", got)
			}

			return
		case <-time.After(250 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			t.Fatal("the watch callback is not triggered by Store")
		}
	}
}

func TestLoadSingleFileWithFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)

	path := filepath.Join(dir, configFile)

	source := file.NewSource(file.WithPath(path), file.WithFiles(configFile))

	configs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("invalid config count, expect: 1, actual: %d", len(configs))
	}

	source = file.NewSource(file.WithPath(path), file.WithFiles("missing.json"))

	configs, err = source.Load(context.Background())
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("invalid config count, expect: 0, actual: %d", len(configs))
	}
}

func TestLoadWithFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)
	writeFile(t, dir, "gate.yaml", "name: gate\n")
	writeFile(t, dir, "node.yaml", "name: node\n")

	tests := []struct {
		name  string
		files []string
		// expect is the set of expected config file names.
		expect []string
	}{
		{name: "exact", files: []string{configFile}, expect: []string{configFile}},
		{name: "pattern", files: []string{"*.yaml"}, expect: []string{"gate.yaml", "node.yaml"}},
		{name: "prefix", files: []string{"gate*"}, expect: []string{"gate.yaml"}},
		{name: "cleaned", files: []string{"./gate.yaml"}, expect: []string{"gate.yaml"}},
		{name: "none", files: []string{"missing.json"}, expect: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := file.NewSource(file.WithPath(dir), file.WithFiles(tt.files...))

			configs, err := source.Load(context.Background())
			if err != nil {
				t.Fatalf("load config failed, err: %v", err)
			}

			actual := make(map[string]struct{}, len(configs))
			for _, cc := range configs {
				actual[cc.File] = struct{}{}
			}

			if len(actual) != len(tt.expect) {
				t.Fatalf("invalid config count, expect: %d, actual: %d", len(tt.expect), len(actual))
			}

			for _, name := range tt.expect {
				if _, ok := actual[name]; !ok {
					t.Fatalf("expect the config file to be loaded, actual: %v", actual)
				}
			}
		})
	}
}

func TestLoadWithFilesExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	gateDir := filepath.Join(dir, "gateway")

	if err := os.MkdirAll(gateDir, 0o755); err != nil {
		t.Fatalf("create directory failed, err: %v", err)
	}

	writeFile(t, gateDir, "gate.yaml", "name: gate\n")
	writeFile(t, gateDir, "node.json", `{"name":"node"}`)

	source := file.NewSource(file.WithPath(dir), file.WithFiles("*.yaml"))

	configs, err := source.Load(context.Background(), "gateway")
	if err != nil {
		t.Fatalf("load the explicit directory failed, err: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("invalid config count, expect: 1, actual: %d", len(configs))
	}
	if configs[0].File != "gate.yaml" {
		t.Fatalf("invalid config file, expect: gate.yaml, actual: %s", configs[0].File)
	}
}

func TestLoadWithFilesRejectsUnlistedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)
	writeFile(t, dir, "gate.yaml", "name: gate\n")

	source := file.NewSource(file.WithPath(dir), file.WithFiles("gate.yaml"))

	if _, err := source.Load(context.Background(), configFile); !errors.Is(err, errors.ErrFileNotConfigured) {
		t.Fatalf("expect ErrFileNotConfigured, actual: %v", err)
	}

	if _, err := source.Load(context.Background(), "gate.yaml"); err != nil {
		t.Fatalf("load the configured file failed, err: %v", err)
	}
}

func TestStoreWithFiles(t *testing.T) {
	dir := t.TempDir()

	source := file.NewSource(file.WithPath(dir), file.WithMode(config.ReadWrite), file.WithFiles("gate.yaml"))

	if err := source.Store(context.Background(), configFile, []byte(`{}`)); !errors.Is(err, errors.ErrFileNotConfigured) {
		t.Fatalf("expect ErrFileNotConfigured, actual: %v", err)
	}

	if err := source.Store(context.Background(), "gate.yaml", []byte(`{"name":"gate"}`)); err != nil {
		t.Fatalf("store the configured file failed, err: %v", err)
	}
}

// TestWatchWithFiles verifies that changes to a file outside the configured list are not reported
// while the listed file is still watched.
func TestWatchWithFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, configFile, `{"timezone":"Local"}`)
	writeFile(t, dir, "gate.yaml", "name: gate\n")

	c := config.NewConfigurator(config.WithSources(file.NewSource(
		file.WithPath(dir),
		file.WithMode(config.ReadWrite),
		file.WithFiles("gate.yaml"),
	)))
	defer c.Close()

	notified := make(chan string, 2)
	c.Watch(func(names ...string) {
		for _, name := range names {
			select {
			case notified <- name:
			default:
			}
		}
	}, "config", "gate")

	writeFile(t, dir, configFile, `{"timezone":"UTC"}`)

	select {
	case name := <-notified:
		t.Fatalf("unexpected notification for the unlisted file, actual: %s", name)
	case <-time.After(700 * time.Millisecond):
	}

	deadline := time.Now().Add(15 * time.Second)

	for {
		writeFile(t, dir, "gate.yaml", "name: gate-2\n")

		select {
		case name := <-notified:
			if name != "gate" {
				t.Fatalf("invalid notified name, expect: gate, actual: %s", name)
			}

			return
		case <-time.After(250 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			t.Fatal("the watch callback is not triggered")
		}
	}
}

// TestWatchNewDirectory verifies that the files that already exist in a directory moved into the
// watched tree are loaded, because a non-recursive watcher cannot observe their creation.
func TestWatchNewDirectory(t *testing.T) {
	dir := t.TempDir()
	staging := t.TempDir()

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
	}, "gate")

	deadline := time.Now().Add(15 * time.Second)

	for {
		sub := filepath.Join(staging, "config")

		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("create directory failed, err: %v", err)
		}
		writeFile(t, sub, "gate.yaml", "name: gate\n")

		target := filepath.Join(dir, "config")
		_ = os.RemoveAll(target)

		if err := os.Rename(sub, target); err != nil {
			t.Fatalf("move directory failed, err: %v", err)
		}

		select {
		case name := <-notified:
			if name != "gate" {
				t.Fatalf("invalid notified name, expect: gate, actual: %s", name)
			}

			return
		case <-time.After(500 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			t.Fatal("the watch callback is not triggered for the new directory")
		}
	}
}
