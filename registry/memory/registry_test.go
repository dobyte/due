package memory

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

func TestSnapshotsAndOwnership(t *testing.T) {
	r := NewRegistry()
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w, err := r.Watch(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	if rows, err := w.Next(); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	ins := &registry.ServiceInstance{ID: "n", Name: "node", Metadata: map[string]string{"key": "old"}, Routes: []registry.Route{{ID: 1}}}
	if err := r.Register(ctx, ins); err != nil {
		t.Fatal(err)
	}
	ins.Metadata["key"] = "mutated"
	ins.Routes[0].ID = 99
	rows, err := w.Next()
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	if rows[0].Metadata["key"] != "old" || rows[0].Routes[0].ID != 1 {
		t.Fatal("input ownership")
	}
	rows[0].Metadata["key"] = "returned"
	rows, err = r.Services(ctx, "node")
	if err != nil || rows[0].Metadata["key"] != "old" {
		t.Fatal("output ownership", err)
	}
	for i := 0; i < 100; i++ {
		ins.Weight = i
		if err := r.Register(ctx, ins); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = w.Next()
	if err != nil || rows[0].Weight != 99 {
		t.Fatal("coalesced snapshot", err)
	}
	if err := r.Deregister(ctx, ins); err != nil {
		t.Fatal(err)
	}
	if rows, err := w.Next(); err != nil || len(rows) != 0 {
		t.Fatal("deregister", rows, err)
	}
}

func TestWatchCancellationAndClose(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	w, err := r.Watch(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := w.Next(); err != nil {
		t.Fatalf("setup cancellation must not stop watcher: %v", err)
	}
	_ = w.Stop()
	w, err = r.Watch(context.Background(), "node")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Next()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Next(); !errors.Is(err, errors.ErrWatcherStopped) {
		t.Fatal(err)
	}
	if _, err := r.Services(context.Background(), "node"); !errors.Is(err, errors.ErrRegistryClosed) {
		t.Fatal(err)
	}
}

func TestOldIncarnationCannotDeregisterReplacement(t *testing.T) {
	r := NewRegistry()
	defer r.Close()
	ctx := context.Background()
	old := &registry.ServiceInstance{ID: "node", Name: "node", Metadata: map[string]string{"due.bridge": "old"}}
	current := &registry.ServiceInstance{ID: "node", Name: "node", Metadata: map[string]string{"due.bridge": "current"}}
	if err := r.Register(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(ctx, current); err != nil {
		t.Fatal(err)
	}
	if err := r.Deregister(ctx, old); err != nil {
		t.Fatal(err)
	}
	rows, err := r.Services(ctx, "node")
	if err != nil || len(rows) != 1 || rows[0].Metadata["due.bridge"] != "current" {
		t.Fatal(rows, err)
	}
}
