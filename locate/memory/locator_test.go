package memory

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
)

func TestConditionalUnbindAndWatch(t *testing.T) {
	l := NewLocator()
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w, err := l.Watch(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	if err := l.BindGate(ctx, 1, "g"); err != nil {
		t.Fatal(err)
	}
	if err := l.BindNode(ctx, 1, "player", "old"); err != nil {
		t.Fatal(err)
	}
	if err := l.BindNode(ctx, 1, "player", "new"); err != nil {
		t.Fatal(err)
	}
	if err := l.UnbindNode(ctx, 1, "player", "old"); err != nil {
		t.Fatal(err)
	}
	if nid, err := l.LocateNode(ctx, 1, "player"); err != nil || nid != "new" {
		t.Fatal(nid, err)
	}
	rows, err := w.Next()
	if err != nil || len(rows) != 2 || rows[0].InsID != "old" || rows[1].InsID != "new" {
		t.Fatal(rows, err)
	}
	nodes, err := l.LocateNodes(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	nodes["player"] = "mutated"
	if nid, _ := l.LocateNode(ctx, 1, "player"); nid != "new" {
		t.Fatal("mutable returned map")
	}
	if err := l.UnbindNode(ctx, 1, "player", "new"); err != nil {
		t.Fatal(err)
	}
	rows, err = w.Next()
	if err != nil || len(rows) != 1 || rows[0].Type != locate.UnbindNode {
		t.Fatal(rows, err)
	}
}

func TestBackpressureRejectsBeforeMutation(t *testing.T) {
	l := NewLocator(1)
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w, err := l.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	if err := l.BindGate(ctx, 1, "first"); err != nil {
		t.Fatal(err)
	}
	if err := l.BindGate(ctx, 1, "second"); !errors.Is(err, errors.ErrWriteTimeout) {
		t.Fatal(err)
	}
	if gid, _ := l.LocateGate(ctx, 1); gid != "first" {
		t.Fatal("partial mutation")
	}
	if _, err := w.Next(); err != nil {
		t.Fatal(err)
	}
	if err := l.BindGate(ctx, 1, "second"); err != nil {
		t.Fatal(err)
	}
	if err := l.UnbindGate(ctx, 1, "first"); err != nil {
		t.Fatal(err)
	}
	if gid, _ := l.LocateGate(ctx, 1); gid != "second" {
		t.Fatal("stale unbind")
	}
	_ = l.Close()
	if _, err := w.Next(); !errors.Is(err, errors.ErrWatcherStopped) {
		t.Fatal(err)
	}
}
