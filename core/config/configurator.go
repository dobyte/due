package config

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dario.cat/mergo"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/jinzhu/copier"
)

const (
	// watchMinBackoff is the initial delay before retrying a failed config watch.
	watchMinBackoff = time.Second
	// watchMaxBackoff is the maximum delay before retrying a failed config watch.
	watchMaxBackoff = 30 * time.Second
	// maxSliceIndex is the largest index that Set expands a config slice to, so that a mistyped
	// index cannot allocate an unbounded amount of memory.
	maxSliceIndex = 10000
)

// Configurator manages a set of config sources and exposes the config loaded from them.
//
// The value store is merge-only: a config that a source no longer provides stays available until the
// configurator is recreated. A changed config replaces its previous value as a whole, so a key
// removed from a config file is removed from the store as well.
//
// At construction every source is loaded once and its watcher is started immediately after that load.
// The first successful watcher open does not load the source again, so a change made between a
// source's startup load and its watch registration is only applied when the source reports a later
// event or when the watcher is reopened; a failed open, a stopped watcher and a read error all
// reload the source. The window is kept as short as possible by starting each watcher right after
// its load.
type Configurator interface {
	// Has reports whether a config matching pattern exists.
	Has(pattern string) bool
	// Get returns the config value under pattern, falling back to def when the config is missing.
	// The returned value may reference the internal config maps and slices and must be treated as
	// read-only; modifying it corrupts the snapshot shared by every reader.
	Get(pattern string, def ...any) value.Value
	// Set sets the config value under pattern; a non-container value on the path is replaced when a
	// nested pattern is given.
	Set(pattern string, value any) error
	// Match returns a [Matcher] over the given patterns.
	Match(patterns ...string) Matcher
	// Watch registers cb to be called when one of the named configs changes; empty names means every
	// config. Callbacks run on a single goroutine and are never invoked concurrently; changes that
	// arrive while a callback runs are merged into the next call.
	Watch(cb WatchCallbackFunc, names ...string)
	// Load loads the configurations of the named source and returns them without applying them to
	// the value store. Only the first non-empty file is loaded; omitting file or passing an empty
	// string loads every file of the source, and any further entries are ignored.
	Load(ctx context.Context, source string, file ...string) ([]*Configuration, error)
	// Store saves content as file under the named source, merging it with the existing config unless
	// override is true.
	Store(ctx context.Context, source string, file string, content any, override ...bool) error
	// Close closes the configurator and its config sources. It waits for a running callback and for
	// the in-flight Load and Store calls to finish, discards the pending notifications, and reports
	// [errors.ErrClosedConfigurator] from the later Set, Load and Store calls. An attempt to register
	// a watcher after Close has no effect, while Get and Has keep serving the last published
	// snapshot. A callback must not call Close.
	Close()
}

// WatchCallbackFunc is called with the names of the configs that changed. The names are sorted and
// may combine several changes.
type WatchCallbackFunc func(names ...string)

type watcher struct {
	names    map[string]struct{}
	callback WatchCallbackFunc
}

type defaultConfigurator struct {
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	sources  map[string]Source
	mu       sync.Mutex
	values   atomic.Pointer[map[string]any]
	rw       sync.RWMutex
	watchers []*watcher
	notifyMu sync.Mutex
	pending  map[string]struct{}
	chNotify chan struct{}
	opsMu    sync.RWMutex
	reloadMu sync.Mutex
	reloads  map[string]*reloadState
	closed   atomic.Bool
	wg       sync.WaitGroup
}

// reloadState tracks the background reload retry of one source. pending records that another retry
// was requested while the current run was finishing, so the worker runs again instead of exiting.
type reloadState struct {
	pending bool
}

var _ Configurator = &defaultConfigurator{}

// NewConfigurator returns a new [Configurator] configured with opts. It loads every source once,
// starts watching the source immediately after its load, and then returns. See [Configurator] for
// the startup reload semantics.
//
// The caller installs the logger with [WithLogger]; log records are discarded when no logger is
// provided. The public config package installs the framework logger instead, and etc installs the
// standard logger.
func NewConfigurator(opts ...Option) Configurator {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	o.normalize()
	o.sources = normalizeSources(o.sources, o)

	r := &defaultConfigurator{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(o.ctx)
	r.watchers = make([]*watcher, 0)
	r.pending = make(map[string]struct{})
	r.reloads = make(map[string]*reloadState)
	r.chNotify = make(chan struct{}, 1)
	r.wg.Go(r.notifyLoop)
	r.init()

	return r
}

// normalizeSources keeps the first source of every name and closes later duplicate instances, so
// that a duplicate source cannot silently shadow another one or leak its connection. Passing the
// same instance twice keeps it open.
func normalizeSources(sources []Source, opts *options) []Source {
	if len(sources) == 0 {
		return nil
	}

	var (
		seen   = make(map[string]Source, len(sources))
		unique = make([]Source, 0, len(sources))
	)

	for _, s := range sources {
		if s == nil {
			opts.warnf("nil config source is ignored")
			continue
		}

		name := s.Name()
		if kept, ok := seen[name]; ok {
			opts.warnf("duplicate config source %q is ignored", name)

			// The same instance may be given twice; closing it would close the source that is kept.
			if !sameSource(kept, s) {
				if err := s.Close(); err != nil {
					opts.errorf("close duplicate config source %q failed: %v", name, err)
				}
			}

			continue
		}

		seen[name] = s
		unique = append(unique, s)
	}

	return unique
}

// sameSource reports whether a and b are the same instance. Pointer sources, which is the shape every
// built-in source uses, are compared by identity; other comparable values are compared by value.
func sameSource(a, b Source) bool {
	if a == nil || b == nil {
		return false
	}

	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if va.Type() != vb.Type() {
		return false
	}

	if va.Kind() == reflect.Pointer {
		return va.Pointer() == vb.Pointer()
	}

	// Comparing interface values whose dynamic type is not comparable panics.
	if !va.Comparable() {
		return false
	}

	return a == b
}

// init initializes the config sources, loads their config into the value store and starts a watcher
// per source. A source that fails to load is retried in the background, so that a temporary failure
// does not leave the store permanently incomplete.
func (c *defaultConfigurator) init() {
	c.sources = make(map[string]Source, len(c.opts.sources))
	for _, s := range c.opts.sources {
		c.sources[s.Name()] = s
	}

	for _, s := range c.opts.sources {
		cs, err := s.Load(c.ctx)
		if err != nil {
			if errors.Is(err, errors.ErrClosedConfigSource) {
				c.opts.warnf("config source %q is closed, skip loading", s.Name())

				continue
			}

			c.opts.errorf("load configure of source %q failed: %v", s.Name(), err)

			c.scheduleReload(s)

			// The startup load failed, so the watcher reloads the source when it opens.
			c.wg.Go(func() {
				c.watchSource(s, true)
			})

			continue
		}

		c.apply(cs)

		// Start the watcher right after the load so that the window in which a change can be missed
		// between the load and the watch registration stays short. The first open does not load the
		// source again; every later open reloads it.
		c.wg.Go(func() {
			c.watchSource(s, false)
		})
	}
}

// reload retries loading the configurations of a source and merges them into the store once the
// source becomes available. It backs off between attempts and logs only the first failure of a
// streak.
func (c *defaultConfigurator) reload(s Source) {
	var (
		delay  = watchMinBackoff
		warned bool
	)

	for {
		if !c.sleep(delay) {
			return
		}

		cs, err := s.Load(c.ctx)
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}

			if errors.Is(err, errors.ErrClosedConfigSource) {
				c.opts.warnf("config source %q is closed, stop reloading", s.Name())

				return
			}

			if !warned {
				c.opts.errorf("reload configure of source %q failed: %v", s.Name(), err)
				warned = true
			}

			delay = nextWatchDelay(delay)

			continue
		}

		if warned {
			c.opts.infof("config source %q reload recovered", s.Name())
		}

		if names := c.apply(cs); len(names) > 0 {
			c.scheduleNotify(names)
		}

		return
	}
}

// reloadRunning reports whether a background reload retry is running for the source.
func (c *defaultConfigurator) reloadRunning(s Source) bool {
	c.reloadMu.Lock()
	_, ok := c.reloads[s.Name()]
	c.reloadMu.Unlock()

	return ok
}

// scheduleReload starts a background reload retry for the source unless one is already running, so
// that a failed load can recover without blocking the watcher of the source. A request that arrives
// while a retry is finishing makes the worker run again, so no request is lost in the handoff.
func (c *defaultConfigurator) scheduleReload(s Source) {
	c.reloadMu.Lock()
	state, ok := c.reloads[s.Name()]
	if ok {
		state.pending = true
		c.reloadMu.Unlock()

		return
	}

	state = &reloadState{}
	c.reloads[s.Name()] = state
	c.reloadMu.Unlock()

	c.wg.Go(func() {
		for {
			c.reload(s)

			c.reloadMu.Lock()
			if c.ctx.Err() != nil || !state.pending {
				delete(c.reloads, s.Name())
				c.reloadMu.Unlock()

				return
			}

			state.pending = false
			c.reloadMu.Unlock()
		}
	})
}

// apply decodes the given configurations and stores them in the value store. The value of every
// applied config replaces its previous value as a whole, so a key removed from a changed config is
// removed from the store. It returns the names of the configurations whose value changed, so that
// reloading unchanged configs does not notify the watchers.
func (c *defaultConfigurator) apply(cs []*Configuration) []string {
	values := make(map[string]any, len(cs))

	for _, cc := range cs {
		if cc.Name == "" {
			c.opts.warnf("config name is empty, the config is ignored")

			continue
		}

		if len(cc.Content) == 0 {
			continue
		}

		v, err := c.opts.decoder(cc.Format, cc.Content)
		if err != nil {
			if !errors.Is(err, errors.ErrInvalidFormat) {
				c.opts.errorf("decode configure failed: %v", err)
			}
			continue
		}

		values[cc.Name] = v
	}

	if len(values) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	dst := c.shallowCopy()
	changed := make([]string, 0, len(values))

	for name, v := range values {
		if current, ok := dst[name]; !ok || !reflect.DeepEqual(current, v) {
			changed = append(changed, name)
		}

		dst[name] = v
	}

	c.store(dst)

	return changed
}

// store publishes values as the current snapshot. The map must not be modified afterwards, so that
// readers loading the snapshot never observe a concurrent write.
func (c *defaultConfigurator) store(values map[string]any) {
	c.values.Store(&values)
}

// load returns the current snapshot.
func (c *defaultConfigurator) load() map[string]any {
	if values := c.values.Load(); values != nil {
		return *values
	}

	return nil
}

// cloneValue returns a deep copy of a decoded config value, so that a writer can modify the copy
// without modifying the published snapshot. Scalars are returned as is, because writing them always
// replaces the value instead of mutating it in place.
func cloneValue(value any) (any, error) {
	switch value.(type) {
	case map[string]any:
		dst := make(map[string]any)
		if err := copier.CopyWithOption(&dst, value, copier.Option{DeepCopy: true}); err != nil {
			return nil, err
		}

		return dst, nil
	case []any:
		dst := make([]any, 0)
		if err := copier.CopyWithOption(&dst, value, copier.Option{DeepCopy: true}); err != nil {
			return nil, err
		}

		return dst, nil
	default:
		return value, nil
	}
}

// cloneConfig returns a deep copy of the named config when it is a map, so that Store can merge into
// it without modifying the published snapshot. It returns a nil map when the config is absent or is
// not a map.
func (c *defaultConfigurator) cloneConfig(name string) (map[string]any, error) {
	value, ok := c.load()[name]
	if !ok {
		return nil, nil
	}

	m, ok := value.(map[string]any)
	if !ok {
		return nil, nil
	}

	cloned, err := cloneValue(m)
	if err != nil {
		return nil, err
	}

	return cloned.(map[string]any), nil
}

// shallowCopy returns a copy of the top-level value map. The nested values are shared with the
// current snapshot: every writer replaces a value as a whole instead of modifying it in place, so
// the shared subtrees stay immutable.
func (c *defaultConfigurator) shallowCopy() map[string]any {
	values := c.load()

	dst := make(map[string]any, len(values))

	for name, v := range values {
		dst[name] = v
	}

	return dst
}

// writableValues returns a copy of the value store that Set may modify. Only the top-level config
// that keys enter is deep copied, because the traversal never modifies another config; the remaining
// configs are shared with the snapshot and are replaced as a whole on write.
func (c *defaultConfigurator) writableValues(keys []string) (map[string]any, []string, error) {
	values := c.load()

	keys = reviseKeys(keys, values)

	dst := make(map[string]any, len(values))

	for name, v := range values {
		dst[name] = v
	}

	if len(keys) == 0 {
		return dst, keys, nil
	}

	v, ok := dst[keys[0]]
	if !ok {
		return dst, keys, nil
	}

	cloned, err := cloneValue(v)
	if err != nil {
		return nil, nil, err
	}

	dst[keys[0]] = cloned

	return dst, keys, nil
}

// watchSource opens a watcher for the source and consumes its changes until the configurator is
// closed. reloadOnOpen controls whether the first successful open loads the source again: a source
// that was just loaded at startup does not need it. Both a watcher that cannot be opened and one
// that stops are retried with a backoff, so that a temporary failure does not disable hot reload
// permanently.
func (c *defaultConfigurator) watchSource(s Source, reloadOnOpen bool) {
	var (
		delay  = watchMinBackoff
		warned bool
	)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// exec watch
		}

		w, err := s.Watch(c.ctx)
		if err == nil && w == nil {
			err = errors.New("nil config watcher")
		}
		if err != nil {
			// The failed attempt means that changes may have been missed, so the next successful
			// open reloads the source.
			reloadOnOpen = true

			if c.ctx.Err() != nil {
				return
			}

			// A closed source cannot be watched again; retrying would only log the same error on
			// every backoff.
			if errors.Is(err, errors.ErrClosedConfigSource) {
				c.opts.warnf("config source %q is closed, stop watching", s.Name())

				return
			}

			if !warned {
				c.opts.errorf("watching configure change failed: %v", err)
				warned = true
			}

			if !c.sleep(delay) {
				return
			}

			delay = nextWatchDelay(delay)

			continue
		}

		// A background reload already covers the source, so the watcher does not issue a second load
		// for the same failure.
		if reloadOnOpen && !c.reloadRunning(s) {
			// Reload the source after the watcher is registered, so that a change that happened
			// between the previous load and the watch registration is not lost. Unchanged configs
			// are not reported, so reopening a watcher does not notify the watchers again.
			names, loadErr := c.reloadSource(s)
			if loadErr != nil {
				if c.ctx.Err() != nil {
					_ = w.Stop()

					return
				}

				if errors.Is(loadErr, errors.ErrClosedConfigSource) {
					_ = w.Stop()

					c.opts.warnf("config source %q is closed, stop watching", s.Name())

					return
				}

				if !warned {
					c.opts.errorf("reload configure of source %q failed: %v", s.Name(), loadErr)
					warned = true
				}

				// Keep consuming the watcher: changes still arrive while the reload is retried in
				// the background, so a temporarily unreadable source does not disable hot reload.
				c.scheduleReload(s)
			} else if len(names) > 0 {
				c.scheduleNotify(names)
			}
		}

		// Every open after this one has to reload the source again.
		reloadOnOpen = true

		warned = false

		if !c.consume(w) {
			delay = watchMinBackoff
		}

		if c.ctx.Err() != nil {
			return
		}

		// The watcher has stopped; reopen it after a backoff so that a watcher that fails
		// immediately is not reopened in a tight loop.
		if !c.sleep(delay) {
			return
		}

		delay = nextWatchDelay(delay)
	}
}

// reloadSource loads the configurations of the source and applies them. It returns the names of the
// configs whose value changed.
func (c *defaultConfigurator) reloadSource(s Source) ([]string, error) {
	cs, err := s.Load(c.ctx)
	if err != nil {
		return nil, err
	}

	return c.apply(cs), nil
}

// consume reads the changes of a watcher until it stops or the configurator is closed. Any read
// error ends the watcher as well: the caller reopens it and reloads the source, which restores the
// events that were lost during the failure (for example, an fsnotify overflow). It reports whether
// the watcher ended with a failure, so the caller can keep the backoff growing.
func (c *defaultConfigurator) consume(w Watcher) (errored bool) {
	defer w.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return false
		default:
			// exec watch
		}

		cs, err := w.Next()
		if err != nil {
			// A watcher reports context.Canceled or a stopped error once it has ended, which is not
			// a failure worth logging.
			if c.ctx.Err() == nil &&
				!errors.Is(err, errors.ErrWatcherStopped) &&
				!errors.Is(err, errors.ErrClosedConfigSource) &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, io.EOF) {
				c.opts.errorf("watching configure change failed: %v", err)

				return true
			}

			return false
		}

		if names := c.apply(cs); len(names) > 0 {
			c.scheduleNotify(names)
		}
	}
}

// sleep waits for delay or until the configurator is closed. It reports false when the configurator
// is closed.
func (c *defaultConfigurator) sleep(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-c.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// nextWatchDelay doubles delay and caps it at watchMaxBackoff.
func nextWatchDelay(delay time.Duration) time.Duration {
	if delay >= watchMaxBackoff {
		return watchMaxBackoff
	}

	if delay *= 2; delay > watchMaxBackoff {
		delay = watchMaxBackoff
	}

	return delay
}

// scheduleNotify records the given names and wakes the notifier without blocking, so that a slow
// callback cannot stall the config sources.
func (c *defaultConfigurator) scheduleNotify(names []string) {
	c.notifyMu.Lock()
	for _, name := range names {
		c.pending[name] = struct{}{}
	}
	c.notifyMu.Unlock()

	select {
	case c.chNotify <- struct{}{}:
	default:
		// A pending notification signal is already queued, and the names are merged into the next
		// callback by notifyLoop.
	}
}

// notifyLoop delivers pending config names on a single goroutine until the configurator is closed.
// Callbacks are therefore never invoked concurrently, and changes that arrive while a callback runs
// are merged into the next call.
func (c *defaultConfigurator) notifyLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.chNotify:
		}

		if names := c.takePending(); len(names) > 0 {
			c.notify(names...)
		}
	}
}

// takePending drains the pending names in a stable order, so that a callback always observes the
// same argument order for the same set of changes.
func (c *defaultConfigurator) takePending() []string {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()

	if len(c.pending) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.pending))
	for name := range c.pending {
		names = append(names, name)
	}

	clear(c.pending)

	slices.Sort(names)

	return names
}

// notify calls every watcher that matches one of the given names. The watcher list is snapshotted
// before the callbacks run, so a callback can call [Configurator.Watch] without deadlocking. A
// panicking callback is recovered so that it cannot terminate the process.
func (c *defaultConfigurator) notify(names ...string) {
	c.rw.RLock()
	watchers := make([]*watcher, len(c.watchers))
	copy(watchers, c.watchers)
	c.rw.RUnlock()

	for _, w := range watchers {
		if len(w.names) == 0 {
			c.callWatchCallback(w.callback, names...)
		} else {
			validNames := make([]string, 0, min(len(w.names), len(names)))
			for _, name := range names {
				if _, ok := w.names[name]; ok {
					validNames = append(validNames, name)
				}
			}

			if len(validNames) > 0 {
				c.callWatchCallback(w.callback, validNames...)
			}
		}
	}
}

// callWatchCallback invokes cb and recovers from its panic, so that a faulty watcher callback cannot
// terminate the process.
func (c *defaultConfigurator) callWatchCallback(cb WatchCallbackFunc, names ...string) {
	defer func() {
		if err := recover(); err != nil {
			c.opts.errorf("watch callback panic: %v", err)
		}
	}()

	cb(names...)
}

// Close closes the configurator and its config sources. It waits for a running callback and for the
// in-flight Load and Store calls to finish, and discards the notifications that are still pending.
func (c *defaultConfigurator) Close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}

	// Wait for the in-flight Load and Store calls before the sources are closed, so that a client is
	// never destroyed under a running request.
	c.opsMu.Lock()

	c.cancel()

	for _, source := range c.sources {
		_ = source.Close()
	}

	c.opsMu.Unlock()

	c.wg.Wait()
}

// Has reports whether a config matching pattern exists.
func (c *defaultConfigurator) Has(pattern string) bool {
	return c.doHas(pattern)
}

// doHas reports whether pattern resolves to a value in the loaded config.
func (c *defaultConfigurator) doHas(pattern string) bool {
	var (
		keys   = strings.Split(pattern, ".")
		node   any
		found  = true
		values = c.load()
	)

	keys = reviseKeys(keys, values)
	node = values
	for _, key := range keys {
		switch vs := node.(type) {
		case map[string]any:
			if v, ok := vs[key]; ok {
				node = v
			} else {
				found = false
			}
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(vs) {
				found = false
			} else {
				node = vs[i]
			}
		default:
			found = false
		}

		if !found {
			break
		}
	}

	return found
}

// Get returns the config value under pattern, falling back to def when the config is missing. The
// returned value must be treated as read-only, since it may reference the internal config maps and
// slices.
func (c *defaultConfigurator) Get(pattern string, def ...any) value.Value {
	if val, ok := c.doGet(pattern); ok {
		return val
	}

	return value.NewValue(def...)
}

// Match returns a [Matcher] over the given patterns.
func (c *defaultConfigurator) Match(patterns ...string) Matcher {
	return &defaultMatcher{c: c, patterns: patterns}
}

// doGet resolves pattern against the loaded values.
func (c *defaultConfigurator) doGet(pattern string) (value.Value, bool) {
	var (
		keys   = strings.Split(pattern, ".")
		node   any
		found  = true
		values = c.load()
	)

	if len(values) == 0 {
		goto NOTFOUND
	}

	keys = reviseKeys(keys, values)
	node = values
	for _, key := range keys {
		switch vs := node.(type) {
		case map[string]any:
			if v, ok := vs[key]; ok {
				node = v
			} else {
				found = false
			}
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(vs) {
				found = false
			} else {
				node = vs[i]
			}
		default:
			found = false
		}

		if !found {
			break
		}
	}

	if found {
		return value.NewValue(node), true
	}

NOTFOUND:
	return nil, false
}

// Set sets the config value under pattern.
func (c *defaultConfigurator) Set(pattern string, value any) error {
	if c.closed.Load() {
		return errors.ErrClosedConfigurator
	}

	var (
		keys = strings.Split(pattern, ".")
		node any
	)

	c.mu.Lock()
	defer c.mu.Unlock()

	values, keys, err := c.writableValues(keys)
	if err != nil {
		return err
	}

	node = values
	for i, key := range keys {
		switch vs := node.(type) {
		case map[string]any:
			if i == len(keys)-1 {
				vs[key] = value
			} else {
				rebuild := false
				ii, err := strconv.Atoi(keys[i+1])
				if next, ok := vs[key]; ok {
					switch nv := next.(type) {
					case map[string]any:
						rebuild = err == nil
					case []any:
						rebuild = err != nil
						// the next node capacity is not enough
						// expand capacity
						if err == nil && ii >= len(nv) {
							if ii > maxSliceIndex {
								return errors.ErrIndexTooLarge
							}

							dst := make([]any, ii+1)
							copy(dst, nv)
							vs[key] = dst
						}
					default:
						rebuild = true
					}
				} else {
					rebuild = true
				}

				if rebuild {
					if err != nil {
						vs[key] = make(map[string]any)
					} else {
						// The new slice has to be long enough for the index that is set next,
						// otherwise the next lookup reports an overflow.
						if ii < 0 {
							return errors.ErrInvalidIndex
						}

						if ii > maxSliceIndex {
							return errors.ErrIndexTooLarge
						}

						vs[key] = make([]any, ii+1)
					}
				}

				node = vs[key]
			}
		case []any:
			ii, err := strconv.Atoi(key)
			if err != nil {
				return err
			}

			if ii < 0 {
				return errors.ErrInvalidIndex
			}

			if ii >= len(vs) {
				return errors.ErrIndexOverflow
			}

			if i == len(keys)-1 {
				vs[ii] = value
			} else {
				rebuild := false
				next, parseErr := strconv.Atoi(keys[i+1])
				switch nv := vs[ii].(type) {
				case map[string]any:
					rebuild = parseErr == nil
				case []any:
					rebuild = parseErr != nil
					// the next node capacity is not enough
					// expand capacity
					if parseErr == nil && next >= len(nv) {
						if next > maxSliceIndex {
							return errors.ErrIndexTooLarge
						}

						dst := make([]any, next+1)
						copy(dst, nv)
						vs[ii] = dst
					}
				default:
					rebuild = true
				}

				if rebuild {
					if parseErr != nil {
						vs[ii] = make(map[string]any)
					} else {
						if next < 0 {
							return errors.ErrInvalidIndex
						}

						if next > maxSliceIndex {
							return errors.ErrIndexTooLarge
						}

						vs[ii] = make([]any, next+1)
					}
				}

				node = vs[ii]
			}
		}
	}

	c.store(values)

	return nil
}

// Watch registers cb to be called when one of the named configs changes; empty names means every
// config.
func (c *defaultConfigurator) Watch(cb WatchCallbackFunc, names ...string) {
	if c.closed.Load() {
		return
	}

	w := &watcher{}
	w.names = make(map[string]struct{}, len(names))
	w.callback = cb

	for _, name := range names {
		w.names[name] = struct{}{}
	}

	c.rw.Lock()
	c.watchers = append(c.watchers, w)
	c.rw.Unlock()
}

// Load loads the configurations of the named source. Only the first non-empty file is loaded;
// omitting file or passing an empty string loads every file of the source, and any further entries
// are ignored. It reports [errors.ErrClosedConfigurator] once the configurator has been closed.
func (c *defaultConfigurator) Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	c.opsMu.RLock()
	defer c.opsMu.RUnlock()

	if c.closed.Load() {
		return nil, errors.ErrClosedConfigurator
	}

	s, ok := c.sources[source]
	if !ok {
		return nil, errors.ErrNotFoundConfigSource
	}

	configs, err := s.Load(ctx, file...)
	if err != nil {
		return nil, err
	}

	for _, cc := range configs {
		cc.decoder, cc.scanner = c.opts.decoder, c.opts.scanner
	}

	return configs, nil
}

// Store saves content as file under the named source, merging it with the existing config unless
// override is true. It reports [errors.ErrClosedConfigurator] once the configurator has been closed.
func (c *defaultConfigurator) Store(ctx context.Context, source string, file string, content any, override ...bool) error {
	c.opsMu.RLock()
	defer c.opsMu.RUnlock()

	if c.closed.Load() {
		return errors.ErrClosedConfigurator
	}

	if content == nil {
		return errors.ErrInvalidConfigContent
	}

	s, ok := c.sources[source]
	if !ok {
		return errors.ErrNotFoundConfigSource
	}

	var (
		err    error
		buf    []byte
		ext    = filepath.Ext(file)
		format = strings.TrimPrefix(ext, ".")
	)

	switch rk, _ := xreflect.Value(content); rk {
	case reflect.Map, reflect.Struct:
		if len(override) > 0 && override[0] {
			buf, err = c.opts.encoder(format, content)
		} else {
			name := strings.TrimSuffix(filepath.Base(file), ext)

			v, cloneErr := c.cloneConfig(name)
			if cloneErr != nil {
				return cloneErr
			}

			if v == nil {
				buf, err = c.opts.encoder(format, content)
			} else {
				buf, err = c.opts.encoder(format, content)
				if err != nil {
					return err
				}

				maps, decodeErr := c.opts.decoder(format, buf)
				if decodeErr != nil {
					return decodeErr
				}

				if err = mergo.Merge(&v, maps, mergo.WithOverride); err != nil {
					return err
				}

				buf, err = c.opts.encoder(format, v)
			}
		}
	case reflect.Array, reflect.Slice:
		buf, err = c.opts.encoder(format, content)
	default:
		buf = xconv.Bytes(xconv.String(content))
	}
	if err != nil {
		return err
	}

	return s.Store(ctx, file, buf)
}

func reviseKeys(keys []string, values map[string]any) []string {
	for i := 1; i < len(keys); i++ {
		key := strings.Join(keys[:i+1], ".")
		if _, ok := values[key]; ok {
			keys[0] = key
			temp := keys[i+1:]
			copy(keys[1:], temp)
			keys = keys[:len(temp)+1]
			break
		}
	}

	return keys
}
