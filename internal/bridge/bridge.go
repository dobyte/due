// Package bridge implements bounded, process-local cluster delivery.
package bridge

import (
	"context"
	"crypto/rand"
	"maps"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

// MetadataKey identifies an instance incarnation, not an authentication credential.
const MetadataKey = "due.bridge"

// Directory isolates local endpoints. The default directory is process-wide.
type Directory struct {
	mu      sync.RWMutex
	prefix  string
	entries map[string]*Endpoint
}

var (
	_default = NewDirectory()
	// Retain common packet sizes without pooling unusually large payloads.
	_copyPool = buffer.NewBytesPoolWithCapacity(64 << 10)
)

func NewDirectory() *Directory {
	return &Directory{entries: make(map[string]*Endpoint)}
}

func Default() *Directory { return _default }

// Endpoint owns its workers and rejects submissions after Close begins.
type Endpoint struct {
	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once
	stop      chan struct{}
	done      chan struct{}
	queues    []chan command
	wg        sync.WaitGroup
	provider  any
	kind      string
	token     string
	directory *Directory
	ctx       context.Context
	cancel    context.CancelFunc
}

type result struct {
	value any
	err   error
}
type command struct {
	run     func(context.Context) (any, error)
	release func()
	result  chan result
}

// Register starts a fixed worker set. Capacity is per shard and bounds queued commands.
func (d *Directory) Register(kind string, provider any, shards, capacity int) *Endpoint {
	if shards < 1 {
		shards = 1
	}
	if capacity < 1 {
		capacity = 1
	}
	d.mu.Lock()
	if d.prefix == "" {
		d.prefix = rand.Text() + "."
	}
	token := d.prefix + rand.Text()
	d.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	e := &Endpoint{provider: provider, kind: kind, token: token,
		directory: d, stop: make(chan struct{}), done: make(chan struct{}),
		queues: make([]chan command, shards), ctx: ctx, cancel: cancel}
	for i := range e.queues {
		q := make(chan command, capacity)
		e.queues[i] = q
		e.wg.Go(func() { e.serve(q) })
	}
	d.mu.Lock()
	d.entries[e.token] = e
	d.mu.Unlock()
	return e
}

func (e *Endpoint) Provider() any { return e.provider }
func (e *Endpoint) Token() string { return e.token }

// Closed reports whether this incarnation has begun stopping.
func (e *Endpoint) Closed() bool { return e.ctx.Err() != nil }

func (e *Endpoint) Metadata(metadata map[string]string) map[string]string {
	out := maps.Clone(metadata)
	if out == nil {
		out = make(map[string]string)
	}
	out[MetadataKey] = e.token
	return out
}

// Lookup distinguishes an absent remote token from a stopped local incarnation.
func (d *Directory) Lookup(token, kind string) (*Endpoint, error) {
	d.mu.RLock()
	e := d.entries[token]
	isLocal := d.prefix != "" && strings.HasPrefix(token, d.prefix)
	d.mu.RUnlock()
	if e != nil && e.kind == kind {
		return e, nil
	}
	if isLocal {
		return nil, errors.ErrServerClosed
	}
	return nil, nil
}

// Target carries discovery metadata through the existing endpoint abstraction.
func Target(raw, token string) string {
	u, err := url.Parse(raw)
	if err != nil || token == "" {
		return raw
	}
	q := u.Query()
	q.Set(MetadataKey, token)
	u.RawQuery = q.Encode()
	return u.String()
}

// Resolve returns a local endpoint or a remote dial address. Local-only targets never dial TCP.
func (d *Directory) Resolve(target, kind string, disabled bool) (*Endpoint, string, error) {
	if !strings.Contains(target, "://") {
		return nil, target, nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, "", err
	}
	if !disabled {
		e, err := d.Lookup(u.Query().Get(MetadataKey), kind)
		if e != nil || err != nil {
			return e, "", err
		}
	}
	if u.Scheme == "local" {
		return nil, "", errors.ErrNotFoundEndpoint
	}
	if u.Scheme != "" {
		return nil, u.Host, nil
	}
	return nil, target, nil
}

// CopyBuffer consumes one reference and returns an independent mutable view.
func CopyBuffer(src buffer.Buffer) buffer.Buffer {
	dst := _copyPool.Get(src.Len())
	if dst == nil {
		dst = buffer.NewBytesWithCapacity(src.Len())
	}
	if src.Nodes() == 1 {
		copy(dst.Bytes(), src.Bytes())
	} else {
		remaining := dst.Bytes()
		src.VisitBytes(func(part []byte) bool {
			remaining = remaining[copy(remaining, part):]
			return true
		})
	}
	src.Release()
	return dst
}

// Submit consumes release on rejection; run owns the payload after acceptance.
// A timed-out caller does not revoke an already accepted operation.
func (e *Endpoint) Submit(ctx context.Context, key uint64, timeout time.Duration, ack bool,
	run func(context.Context) (any, error), release func(), writeTimeout ...time.Duration) (any, error) {
	admission := timeout
	if len(writeTimeout) > 0 {
		admission = writeTimeout[0]
	}
	if ack && timeout > 0 {
		bounded, cancel := context.WithTimeout(ctx, timeout)
		value, err := e.submit(bounded, key, admission, ack, run, release)
		cancel()
		return value, err
	}
	return e.submit(ctx, key, admission, ack, run, release)
}

func (e *Endpoint) submit(ctx context.Context, key uint64, writeTimeout time.Duration, ack bool,
	run func(context.Context) (any, error), release func()) (any, error) {
	c := command{run: run, release: release}
	if ack {
		c.result = make(chan result, 1)
	}
	if err := ctx.Err(); err != nil {
		if release != nil {
			release()
		}
		return nil, err
	}
	if err := e.enqueue(ctx, key, writeTimeout, c); err != nil {
		if release != nil {
			release()
		}
		return nil, err
	}
	if !ack {
		return nil, nil
	}
	select {
	case res := <-c.result:
		return res.value, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Endpoint) enqueue(ctx context.Context, key uint64, writeTimeout time.Duration,
	c command) error {
	e.mu.RLock()
	if e.closed || e.ctx.Err() != nil {
		e.mu.RUnlock()
		return errors.ErrServerClosed
	}
	q := e.queues[key%uint64(len(e.queues))]
	// A timer is needed only when admission actually waits for queue capacity.
	select {
	case q <- c:
		e.mu.RUnlock()
		return nil
	case <-ctx.Done():
		e.mu.RUnlock()
		return ctx.Err()
	case <-e.stop:
		e.mu.RUnlock()
		return errors.ErrServerClosed
	default:
	}
	admission := ctx
	var cancel context.CancelFunc
	if writeTimeout > 0 {
		admission, cancel = context.WithTimeout(ctx, writeTimeout)
	}
	var err error
	select {
	case q <- c:
	case <-admission.Done():
		err = ctx.Err()
		if err == nil {
			err = errors.ErrWriteTimeout
		}
	case <-e.stop:
		err = errors.ErrServerClosed
	}
	e.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	return err
}

func (e *Endpoint) serve(q <-chan command) {
	for c := range q {
		if e.ctx.Err() != nil {
			if c.release != nil {
				c.release()
			}
			if c.result != nil {
				c.result <- result{err: errors.ErrServerClosed}
			}
			continue
		}
		v, err := c.run(e.ctx)
		if c.result != nil {
			c.result <- result{value: v, err: err}
		} else if err != nil {
			log.Warnf("local cluster delivery failed: %v", err)
		}
	}
}

// Close cancels providers, releases queued commands, and joins workers before returning.
// Providers must honor cancellation or have bounded execution.
func (e *Endpoint) Close() {
	e.directory.mu.Lock()
	delete(e.directory.entries, e.token)
	e.directory.mu.Unlock()
	// Closing stop must unblock producers before taking the exclusive queue lock.
	e.cancel()
	e.close()
}

func (e *Endpoint) close() {
	// Cancelled context unblocks submissions waiting on a full queue via stop.
	// Stop is closed exactly once by closeOnce.
	e.closeOnce.Do(func() {
		close(e.stop)
		e.mu.Lock()
		e.closed = true
		for _, q := range e.queues {
			close(q)
		}
		e.mu.Unlock()
		e.wg.Wait()
		close(e.done)
	})
	<-e.done
}
