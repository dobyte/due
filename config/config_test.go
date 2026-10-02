package config_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
)

// fakeWatcher is an in-memory config.Watcher whose events are pushed by the test.
type fakeWatcher struct {
	ctx context.Context
	ch  chan []*config.Configuration
}

// Next blocks until an event is pushed or the context is canceled.
func (w *fakeWatcher) Next() ([]*config.Configuration, error) {
	select {
	case cs := <-w.ch:
		return cs, nil
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	}
}

func (w *fakeWatcher) Stop() error {
	return nil
}

// fakeSource is an in-memory config.Source used to drive the configurator deterministically.
type fakeSource struct {
	name    string
	format  string
	content []byte
	events  chan []*config.Configuration

	mu     sync.Mutex
	stored map[string][]byte
	closed bool
}

func newFakeSource(content string) *fakeSource {
	return &fakeSource{
		name:    "fake",
		format:  "json",
		content: []byte(content),
		events:  make(chan []*config.Configuration, 8),
		stored:  make(map[string][]byte),
	}
}

func (s *fakeSource) Name() string {
	return s.name
}

func (s *fakeSource) Load(_ context.Context, _ ...string) ([]*config.Configuration, error) {
	if s.content == nil {
		return nil, nil
	}

	return []*config.Configuration{{
		Name:    "config",
		Format:  s.format,
		Content: s.content,
	}}, nil
}

func (s *fakeSource) Store(_ context.Context, file string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stored[file] = content

	return nil
}

func (s *fakeSource) Watch(ctx context.Context) (config.Watcher, error) {
	return &fakeWatcher{ctx: ctx, ch: s.events}, nil
}

func (s *fakeSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	return nil
}

// push emits a configuration change event to every watcher of the source.
func (s *fakeSource) push(cs ...*config.Configuration) {
	s.events <- cs
}

// storedContent returns the content stored under the given file name.
func (s *fakeSource) storedContent(file string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.stored[file]
}

// waitNames waits until the given name shows up in a notification, failing the test on timeout.
func waitNames(t *testing.T, ch <-chan []string, want string) {
	t.Helper()

	deadline := time.After(5 * time.Second)

	for {
		select {
		case names := <-ch:
			for _, name := range names {
				if name == want {
					return
				}
			}
		case <-deadline:
			t.Fatalf("timeout waiting for the %q notification", want)
		}
	}
}

func TestConfiguratorGet(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local","nested":{"a":1},"list":[10,20]}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	if !c.Has("config.timezone") {
		t.Fatal("expect `config.timezone` to exist")
	}
	if c.Has("config.missing") {
		t.Fatal("expect `config.missing` not to exist")
	}

	tests := []struct {
		pattern string
		expect  string
	}{
		{pattern: "config.timezone", expect: "Local"},
		{pattern: "config.nested.a", expect: "1"},
		{pattern: "config.list.1", expect: "20"},
	}

	for _, tt := range tests {
		if got := c.Get(tt.pattern).String(); got != tt.expect {
			t.Fatalf("invalid value of %s, expect: %s, actual: %s", tt.pattern, tt.expect, got)
		}
	}

	if got := c.Get("config.list.9", "fallback").String(); got != "fallback" {
		t.Fatalf("invalid fallback value for the out-of-range index, actual: %s", got)
	}

	if got := c.Get("config.list.x", "fallback").String(); got != "fallback" {
		t.Fatalf("invalid fallback value for the non-numeric index, actual: %s", got)
	}

	if got := c.Get("config.missing", "fallback").String(); got != "fallback" {
		t.Fatalf("invalid fallback value for the missing key, actual: %s", got)
	}
}

func TestConfiguratorSet(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local","list":[1,2]}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	if err := c.Set("config.timezone", "UTC"); err != nil {
		t.Fatalf("set config failed, err: %v", err)
	}
	if got := c.Get("config.timezone").String(); got != "UTC" {
		t.Fatalf("invalid value, expect: UTC, actual: %s", got)
	}

	if err := c.Set("config.addr.host", "127.0.0.1"); err != nil {
		t.Fatalf("set config failed, err: %v", err)
	}
	if got := c.Get("config.addr.host").String(); got != "127.0.0.1" {
		t.Fatalf("invalid value, expect: 127.0.0.1, actual: %s", got)
	}

	if err := c.Set("extra.key", "value"); err != nil {
		t.Fatalf("set config failed, err: %v", err)
	}
	if got := c.Get("extra.key").String(); got != "value" {
		t.Fatalf("invalid value, expect: value, actual: %s", got)
	}

	if err := c.Set("config.list.1", 9); err != nil {
		t.Fatalf("set config failed, err: %v", err)
	}
	if got := c.Get("config.list.1").Int(); got != 9 {
		t.Fatalf("invalid value, expect: 9, actual: %d", got)
	}

	// Growing the slice beyond its current length expands it instead of failing.
	if err := c.Set("config.list.5", 9); err != nil {
		t.Fatalf("set config failed, err: %v", err)
	}
	if got := c.Get("config.list.5").Int(); got != 9 {
		t.Fatalf("invalid value, expect: 9, actual: %d", got)
	}
}

func TestConfiguratorMatch(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	m := c.Match("config.missing", "config.timezone")

	if !m.Has() {
		t.Fatal("expect the matcher to find a config")
	}
	if got := m.Get().String(); got != "Local" {
		t.Fatalf("invalid matched value, expect: Local, actual: %s", got)
	}

	var tz string
	if err := m.Scan(&tz); err != nil {
		t.Fatalf("scan matched value failed, err: %v", err)
	}
	if tz != "Local" {
		t.Fatalf("invalid scanned value, expect: Local, actual: %s", tz)
	}

	empty := c.Match("config.missing")
	if empty.Has() {
		t.Fatal("expect the matcher not to find a config")
	}
	if got := empty.Get("fallback").String(); got != "fallback" {
		t.Fatalf("invalid fallback value, expect: fallback, actual: %s", got)
	}
	if err := empty.Scan(&tz); err != nil {
		t.Fatalf("scan with no match should not fail, err: %v", err)
	}
}

func TestConfiguratorWatch(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	all := make(chan []string, 8)
	filtered := make(chan []string, 8)

	c.Watch(func(names ...string) { all <- names })
	c.Watch(func(names ...string) { filtered <- names }, "gate")

	source.push(&config.Configuration{Name: "config", Format: "json", Content: []byte(`{"timezone":"UTC"}`)})
	waitNames(t, all, "config")

	// The filtered watcher only subscribes to `gate`, so it must not receive the `config` change.
	select {
	case names := <-filtered:
		t.Fatalf("the filtered watcher should not be notified, actual: %v", names)
	case <-time.After(100 * time.Millisecond):
	}

	if got := c.Get("config.timezone").String(); got != "UTC" {
		t.Fatalf("config is not reloaded after the change, actual: %s", got)
	}

	source.push(&config.Configuration{Name: "gate", Format: "json", Content: []byte(`{"name":"gate"}`)})
	waitNames(t, all, "gate")
	waitNames(t, filtered, "gate")
}

func TestConfiguratorLoad(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	configs, err := c.Load(context.Background(), source.Name())
	if err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("invalid config count, expect: 1, actual: %d", len(configs))
	}

	if _, err = configs[0].Decode(); err != nil {
		t.Fatalf("decode config failed, err: %v", err)
	}

	if _, err = c.Load(context.Background(), "missing"); !errors.Is(err, errors.ErrNotFoundConfigSource) {
		t.Fatalf("expect ErrNotFoundConfigSource, actual: %v", err)
	}
}

func TestConfiguratorStore(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	ctx := context.Background()

	if err := c.Store(ctx, source.Name(), "config.json", nil); !errors.Is(err, errors.ErrInvalidConfigContent) {
		t.Fatalf("expect ErrInvalidConfigContent, actual: %v", err)
	}

	if err := c.Store(ctx, "missing", "config.json", map[string]any{"a": 1}); !errors.Is(err, errors.ErrNotFoundConfigSource) {
		t.Fatalf("expect ErrNotFoundConfigSource, actual: %v", err)
	}

	if err := c.Store(ctx, source.Name(), "config.json", map[string]any{"timezone": "UTC"}, true); err != nil {
		t.Fatalf("store config failed, err: %v", err)
	}
	if content := source.storedContent("config.json"); !strings.Contains(string(content), "UTC") {
		t.Fatalf("invalid stored content, actual: %s", content)
	}

	// The merge branch reuses the already loaded config as the base document.
	if err := c.Store(ctx, source.Name(), "config.json", map[string]any{"timezone": "UTC"}); err != nil {
		t.Fatalf("store config failed, err: %v", err)
	}
	if content := source.storedContent("config.json"); !strings.Contains(string(content), "UTC") {
		t.Fatalf("invalid merged content, actual: %s", content)
	}
}

func TestGlobalConfigurator(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)

	config.SetConfigurator(config.NewConfigurator(config.WithSources(source)))
	defer config.Close()

	if config.GetConfigurator() == nil {
		t.Fatal("expect the global configurator to be set")
	}

	if !config.Has("config.timezone") {
		t.Fatal("expect `config.timezone` to exist")
	}
	if got := config.Get("config.timezone").String(); got != "Local" {
		t.Fatalf("invalid value, expect: Local, actual: %s", got)
	}

	if err := config.Set("config.timezone", "UTC"); err != nil {
		t.Fatalf("set config failed, err: %v", err)
	}
	if got := config.Get("config.timezone").String(); got != "UTC" {
		t.Fatalf("invalid value, expect: UTC, actual: %s", got)
	}

	if !config.Match("config.timezone").Has() {
		t.Fatal("expect the matcher to find a config")
	}

	config.Watch(func(...string) {})

	if _, err := config.Load(context.Background(), source.Name()); err != nil {
		t.Fatalf("load config failed, err: %v", err)
	}

	if err := config.Store(context.Background(), source.Name(), "config.json", map[string]any{"a": 1}, true); err != nil {
		t.Fatalf("store config failed, err: %v", err)
	}

	// Replacing the global configurator also closes the previous one.
	config.SetConfiguratorWithSources(source)
}

func TestConfiguratorStoreFormats(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	ctx := context.Background()

	for _, file := range []string{"gate.json", "gate.yaml", "gate.yml", "gate.toml"} {
		if err := c.Store(ctx, source.Name(), file, map[string]any{"name": "gate"}, true); err != nil {
			t.Fatalf("store %s failed, err: %v", file, err)
		}
		if content := source.storedContent(file); len(content) == 0 {
			t.Fatalf("empty content stored for %s", file)
		}
	}

	if err := c.Store(ctx, source.Name(), "config.unknown", map[string]any{"name": "gate"}); !errors.Is(err, errors.ErrInvalidFormat) {
		t.Fatalf("expect ErrInvalidFormat, actual: %v", err)
	}
}

func TestConfiguratorStoreNonMap(t *testing.T) {
	source := newFakeSource(`{"timezone":"Local"}`)
	c := config.NewConfigurator(config.WithSources(source))
	defer c.Close()

	ctx := context.Background()

	if err := c.Store(ctx, source.Name(), "list.json", []int{1, 2, 3}, true); err != nil {
		t.Fatalf("store slice failed, err: %v", err)
	}
	if content := string(source.storedContent("list.json")); content != "[1,2,3]" {
		t.Fatalf("invalid stored slice, actual: %s", content)
	}

	if err := c.Store(ctx, source.Name(), "scalar.txt", "hello"); err != nil {
		t.Fatalf("store scalar failed, err: %v", err)
	}
	if content := string(source.storedContent("scalar.txt")); content != "hello" {
		t.Fatalf("invalid stored scalar, actual: %s", content)
	}
}

func TestConfiguratorWithCustomDecoder(t *testing.T) {
	source := newFakeSource(`{"raw":"1"}`)

	c := config.NewConfigurator(
		config.WithContext(context.Background()),
		config.WithSources(source),
		config.WithDecoder(func(string, []byte) (any, error) {
			return map[string]any{"custom": "yes"}, nil
		}),
	)
	defer c.Close()

	if got := c.Get("config.custom").String(); got != "yes" {
		t.Fatalf("invalid value, expect: yes, actual: %s", got)
	}
}

func TestConfigurationWithoutCodec(t *testing.T) {
	// A configuration that never went through the configurator has no decoder nor scanner attached.
	cc := &config.Configuration{Name: "config", Format: "json", Content: []byte(`{"timezone":"Local"}`)}

	if _, err := cc.Decode(); !errors.Is(err, errors.ErrInvalidDecoder) {
		t.Fatalf("expect ErrInvalidDecoder, actual: %v", err)
	}

	var dst struct{}
	if err := cc.Scan(&dst); !errors.Is(err, errors.ErrInvalidScanner) {
		t.Fatalf("expect ErrInvalidScanner, actual: %v", err)
	}
}
