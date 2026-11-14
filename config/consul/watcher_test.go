package consul

import (
	"context"
	"testing"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
)

// TestWatcherDiff verifies that only new or changed configs are reported and that removed configs
// are dropped from the snapshot.
func TestWatcherDiff(t *testing.T) {
	w := &watcher{configs: make(map[string]*config.Configuration)}

	first := &config.Configuration{FullPath: "config/app.json", Content: []byte(`{"a":1}`)}
	if changed := w.diff([]*config.Configuration{first}); len(changed) != 1 {
		t.Fatalf("expect the new config to be reported, actual: %d", len(changed))
	}

	same := &config.Configuration{FullPath: "config/app.json", Content: []byte(`{"a":1}`)}
	if changed := w.diff([]*config.Configuration{same}); len(changed) != 0 {
		t.Fatalf("expect an unchanged config not to be reported, actual: %d", len(changed))
	}

	updated := &config.Configuration{FullPath: "config/app.json", Content: []byte(`{"a":2}`)}
	if changed := w.diff([]*config.Configuration{updated}); len(changed) != 1 {
		t.Fatalf("expect the changed config to be reported, actual: %d", len(changed))
	}

	if changed := w.diff(nil); len(changed) != 0 {
		t.Fatalf("expect a removal not to be reported, actual: %d", len(changed))
	}

	if len(w.configs) != 0 {
		t.Fatalf("expect the removed config to be dropped from the snapshot, actual: %d", len(w.configs))
	}
}

// TestWatchClosedSource verifies that a closed source rejects new watchers instead of registering a
// watcher that is never stopped.
func TestWatchClosedSource(t *testing.T) {
	s := NewSource()

	if err := s.Close(); err != nil {
		t.Fatalf("close source failed, err: %v", err)
	}

	if _, err := s.Watch(context.Background()); !errors.Is(err, errors.ErrClosedConfigSource) {
		t.Fatalf("expect ErrClosedConfigSource, actual: %v", err)
	}
}
