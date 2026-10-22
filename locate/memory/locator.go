// Package memory provides process-local user bindings and ordered change events.
package memory

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
)

// Locator is shared by all components of a local application. Mutations reject
// with ErrWriteTimeout before changing state if a matching watcher is full.
type Locator struct {
	mu       sync.Mutex
	closed   bool
	gates    map[int64]string
	nodes    map[int64]map[string]string
	watchers map[*watcher]struct{}
	capacity int
}

var _ locate.Locator = (*Locator)(nil)

// NewLocator creates an isolated store. The optional event capacity bounds each
// watcher's pending events; the default is 4096. No events are silently dropped.
func NewLocator(eventCapacity ...int) *Locator {
	capacity := 4096
	if len(eventCapacity) > 0 && eventCapacity[0] > 0 {
		capacity = eventCapacity[0]
	}
	return &Locator{gates: make(map[int64]string), nodes: make(map[int64]map[string]string),
		watchers: make(map[*watcher]struct{}), capacity: capacity}
}

func (l *Locator) Name() string { return "memory" }

func (l *Locator) LocateGate(ctx context.Context, uid int64) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.check(ctx); err != nil {
		return "", err
	}
	return l.gates[uid], nil
}

func (l *Locator) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.check(ctx); err != nil {
		return "", err
	}
	return l.nodes[uid][name], nil
}

func (l *Locator) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.check(ctx); err != nil {
		return nil, err
	}
	return maps.Clone(l.nodes[uid]), nil
}

func (l *Locator) BindGate(ctx context.Context, uid int64, gid string) error {
	return l.change(ctx, locate.Event{UID: uid, InsID: gid, InsKind: cluster.Gate.String(), Type: locate.BindGate})
}

func (l *Locator) BindNode(ctx context.Context, uid int64, name, nid string) error {
	return l.change(ctx, locate.Event{UID: uid, InsID: nid, InsName: name, InsKind: cluster.Node.String(), Type: locate.BindNode})
}

func (l *Locator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	return l.change(ctx, locate.Event{UID: uid, InsID: gid, InsKind: cluster.Gate.String(), Type: locate.UnbindGate})
}

func (l *Locator) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	return l.change(ctx, locate.Event{UID: uid, InsID: nid, InsName: name, InsKind: cluster.Node.String(), Type: locate.UnbindNode})
}

func (l *Locator) change(ctx context.Context, event locate.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.check(ctx); err != nil {
		return err
	}
	if event.Type == locate.UnbindGate && l.gates[event.UID] != event.InsID {
		return nil
	}
	if event.Type == locate.UnbindNode && l.nodes[event.UID][event.InsName] != event.InsID {
		return nil
	}
	for w := range l.watchers {
		if w.matches(event.InsKind) && len(w.pending) >= l.capacity {
			return errors.ErrWriteTimeout
		}
	}
	switch event.Type {
	case locate.BindGate:
		l.gates[event.UID] = event.InsID
	case locate.BindNode:
		if l.nodes[event.UID] == nil {
			l.nodes[event.UID] = make(map[string]string)
		}
		l.nodes[event.UID][event.InsName] = event.InsID
	case locate.UnbindGate:
		delete(l.gates, event.UID)
	case locate.UnbindNode:
		delete(l.nodes[event.UID], event.InsName)
		if len(l.nodes[event.UID]) == 0 {
			delete(l.nodes, event.UID)
		}
	}
	for w := range l.watchers {
		if !w.matches(event.InsKind) {
			continue
		}
		copy := event
		w.pending = append(w.pending, &copy)
		select {
		case w.changed <- struct{}{}:
		default:
		}
	}
	return nil
}

func (l *Locator) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.check(ctx); err != nil {
		return nil, err
	}
	w := &watcher{owner: l, kinds: slices.Clone(kinds), changed: make(chan struct{}, 1), done: make(chan struct{})}
	l.watchers[w] = struct{}{}
	return w, nil
}

func (l *Locator) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	for w := range l.watchers {
		w.stop()
	}
	clear(l.gates)
	clear(l.nodes)
	return nil
}

func (l *Locator) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if l.closed {
		return errors.ErrServerClosed
	}
	return nil
}

type watcher struct {
	owner   *Locator
	kinds   []string
	pending []*locate.Event
	changed chan struct{}
	done    chan struct{}
	stopped bool
}

func (w *watcher) matches(kind string) bool {
	return len(w.kinds) == 0 || slices.Contains(w.kinds, kind)
}

func (w *watcher) Next() ([]*locate.Event, error) {
	for {
		select {
		case <-w.done:
			return nil, errors.ErrWatcherStopped
		case <-w.changed:
		}
		w.owner.mu.Lock()
		if w.stopped {
			w.owner.mu.Unlock()
			return nil, errors.ErrWatcherStopped
		}
		events := w.pending
		w.pending = nil
		w.owner.mu.Unlock()
		if len(events) != 0 {
			return events, nil
		}
	}
}

func (w *watcher) Stop() error {
	w.owner.mu.Lock()
	defer w.owner.mu.Unlock()
	w.stop()
	return nil
}

func (w *watcher) stop() {
	if w.stopped {
		return
	}
	w.stopped = true
	w.pending = nil
	close(w.done)
	delete(w.owner.watchers, w)
}
