package due

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/etc"
)

// mockComponent records the lifecycle calls it receives so that tests can assert on them.
type mockComponent struct {
	name string

	mu     sync.Mutex
	events []string
}

var _ component.Component = (*mockComponent)(nil)

// Name returns the component name.
func (m *mockComponent) Name() string { return m.name }

// record appends a lifecycle event.
func (m *mockComponent) record(event string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)
}

// Init records an init call.
func (m *mockComponent) Init() { m.record("init") }

// Start records a start call.
func (m *mockComponent) Start() { m.record("start") }

// Close records a close call.
func (m *mockComponent) Close() { m.record("close") }

// Destroy records a destroy call.
func (m *mockComponent) Destroy() { m.record("destroy") }

// snapshot returns a copy of the recorded events.
func (m *mockComponent) snapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.events...)
}

// has reports whether the given event was recorded.
func (m *mockComponent) has(event string) bool {
	for _, e := range m.snapshot() {
		if e == event {
			return true
		}
	}

	return false
}

// TestNewContainerAndAdd verifies that components are appended to the container.
func TestNewContainerAndAdd(t *testing.T) {
	c := NewContainer()
	if c == nil {
		t.Fatal("expect a non-nil container")
	}

	a := &mockComponent{name: "a"}
	b := &mockComponent{name: "b"}
	c.Add(a, b)

	if len(c.components) != 2 {
		t.Fatalf("invalid component count, expect: 2, actual: %d", len(c.components))
	}
}

// TestContainerInitAndStartComponents verifies that every component is initialized before it is
// started.
func TestContainerInitAndStartComponents(t *testing.T) {
	c := NewContainer()
	a := &mockComponent{name: "a"}
	b := &mockComponent{name: "b"}
	c.Add(a, b)

	c.initAllComponents()
	c.startAllComponents()

	for _, m := range []*mockComponent{a, b} {
		events := m.snapshot()
		if len(events) != 2 || events[0] != "init" || events[1] != "start" {
			t.Errorf("invalid lifecycle events for %s, actual: %v", m.name, events)
		}
	}
}

// TestContainerCloseAndDestroyComponents verifies that every component is closed and destroyed.
func TestContainerCloseAndDestroyComponents(t *testing.T) {
	c := NewContainer()
	a := &mockComponent{name: "a"}
	b := &mockComponent{name: "b"}
	c.Add(a, b)

	c.closeAllComponents()
	c.destroyAllComponents()

	for _, m := range []*mockComponent{a, b} {
		if !m.has("close") {
			t.Errorf("expect component %s to be closed", m.name)
		}
		if !m.has("destroy") {
			t.Errorf("expect component %s to be destroyed", m.name)
		}
	}
}

// TestContainerPidFile verifies that the pid file is written with the current process id and then
// removed.
func TestContainerPidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "due.pid")
	if err := etc.Set(defaultPIDKey, path); err != nil {
		t.Fatalf("set pid config failed, err: %v", err)
	}
	defer func() { _ = etc.Set(defaultPIDKey, "") }()

	c := NewContainer()
	c.savePidToFile()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pid file failed, err: %v", err)
	}
	if pid, err := strconv.Atoi(string(data)); err != nil || pid != os.Getpid() {
		t.Errorf("invalid pid content, expect: %d, actual: %q", os.Getpid(), data)
	}

	c.deletePidFile()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expect the pid file to be removed, stat err: %v", err)
	}

	// Removing a missing pid file must be a no-op.
	c.deletePidFile()
}

// TestContainerPidFileWithoutPath verifies that the pid helpers do nothing when no pid file is
// configured.
func TestContainerPidFileWithoutPath(t *testing.T) {
	if err := etc.Set(defaultPIDKey, ""); err != nil {
		t.Fatalf("set pid config failed, err: %v", err)
	}

	c := NewContainer()
	c.savePidToFile()
	c.deletePidFile()
}

// TestContainerPrintFrameworkInfo verifies that printing the framework information does not
// panic.
func TestContainerPrintFrameworkInfo(t *testing.T) {
	NewContainer().printFrameworkInfo()
}

// TestContainerServe verifies the whole container lifecycle, skipping the system signal wait.
func TestContainerServe(t *testing.T) {
	c := NewContainer()
	a := &mockComponent{name: "a"}
	b := &mockComponent{name: "b"}
	c.Add(a, b)

	c.Serve(true)

	for _, m := range []*mockComponent{a, b} {
		for _, event := range []string{"init", "start", "close", "destroy"} {
			if !m.has(event) {
				t.Errorf("expect component %s to receive %s", m.name, event)
			}
		}
	}
}
