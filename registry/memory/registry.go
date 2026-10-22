// Package memory provides process-local service discovery. Share one Registry
// between all components in the application and close it after those components.
package memory

import (
	"context"
	"maps"
	"slices"
	"sort"
	"sync"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/bridge"
	"github.com/dobyte/due/v2/registry"
)

type Registry struct {
	mu       sync.Mutex
	closed   bool
	services map[string]map[string]*registry.ServiceInstance
	watchers map[*watcher]struct{}
}

var _ registry.Registry = (*Registry)(nil)

func NewRegistry() *Registry {
	return &Registry{services: make(map[string]map[string]*registry.ServiceInstance),
		watchers: make(map[*watcher]struct{})}
}

func (r *Registry) Name() string { return "memory" }

func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if ins == nil || ins.ID == "" || ins.Name == "" {
		return errors.ErrInvalidArgument
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(ctx); err != nil {
		return err
	}
	if r.services[ins.Name] == nil {
		r.services[ins.Name] = make(map[string]*registry.ServiceInstance)
	}
	r.services[ins.Name][ins.ID] = clone(ins)
	r.notify(ins.Name)
	return nil
}

func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if ins == nil {
		return errors.ErrInvalidArgument
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(ctx); err != nil {
		return err
	}
	if current := r.services[ins.Name][ins.ID]; current != nil {
		oldToken, newToken := ins.Metadata[bridge.MetadataKey], current.Metadata[bridge.MetadataKey]
		if oldToken != "" && newToken != "" && oldToken != newToken {
			return nil
		}
	}
	delete(r.services[ins.Name], ins.ID)
	if len(r.services[ins.Name]) == 0 {
		delete(r.services, ins.Name)
	}
	r.notify(ins.Name)
	return nil
}

func (r *Registry) Services(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	return r.snapshot(name), nil
}

func (r *Registry) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	w := &watcher{owner: r, name: name, changed: make(chan struct{}, 1), done: make(chan struct{})}
	r.watchers[w] = struct{}{}
	w.changed <- struct{}{} // Initial snapshot and subscription share the same lock.
	return w, nil
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	for w := range r.watchers {
		w.stop()
	}
	clear(r.services)
	return nil
}

func (r *Registry) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.closed {
		return errors.ErrRegistryClosed
	}
	return nil
}

func (r *Registry) notify(name string) {
	for w := range r.watchers {
		if w.name == name {
			select {
			case w.changed <- struct{}{}:
			default:
			}
		}
	}
}

func (r *Registry) snapshot(name string) []*registry.ServiceInstance {
	var out []*registry.ServiceInstance
	for _, ins := range r.services[name] {
		out = append(out, clone(ins))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func clone(ins *registry.ServiceInstance) *registry.ServiceInstance {
	out := *ins
	out.Events = slices.Clone(ins.Events)
	out.Routes = slices.Clone(ins.Routes)
	out.Services = slices.Clone(ins.Services)
	out.Metadata = maps.Clone(ins.Metadata)
	return &out
}

type watcher struct {
	owner   *Registry
	name    string
	changed chan struct{}
	done    chan struct{}
	stopped bool
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	select {
	case <-w.done:
		return nil, errors.ErrWatcherStopped
	case <-w.changed:
	}
	w.owner.mu.Lock()
	defer w.owner.mu.Unlock()
	if w.stopped {
		return nil, errors.ErrWatcherStopped
	}
	return w.owner.snapshot(w.name), nil
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
	close(w.done)
	delete(w.owner.watchers, w)
}
