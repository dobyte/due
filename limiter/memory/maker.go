// Package memory provides an in-process limiter backend.
//
// It implements the same token bucket semantics as the redis backend and is intended for
// single-process scenarios, tests and local degradation. Buckets are kept in a sharded map and
// expired buckets are evicted periodically.
package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	corelimiter "github.com/dobyte/due/v2/core/limiter"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
)

const shardCount = 32

// Maker is a memory limiter maker.
type Maker struct {
	opts   *options
	shards [shardCount]*shard
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed atomic.Bool
}

var _ limiter.Maker = (*Maker)(nil)

// NewMaker creates a memory limiter maker and starts the expired bucket cleanup.
//
// A cleanup interval of zero or less is clamped to the default of one minute. The cleanup
// goroutine stops when Close is called.
func NewMaker(opts ...Option) *Maker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.cleanupInterval <= 0 {
		o.cleanupInterval = defaultCleanupInterval
	}

	m := &Maker{opts: o}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	for i := range shardCount {
		m.shards[i] = &shard{buckets: make(map[string]*bucket)}
	}

	m.wg.Add(1)
	go m.cleanup()

	return m
}

// Make creates a limiter for the named scene and rule.
//
// The name identifies the scene and must be stable for the lifetime of the maker; reusing the
// same name with a different rule shares the existing bucket state.
func (m *Maker) Make(name string, rule limiter.Rule) (limiter.Limiter, error) {
	if m.closed.Load() {
		return nil, errors.ErrIllegalOperation
	}

	if name == "" {
		return nil, errors.ErrInvalidLimiterRule
	}

	normalized, err := rule.Normalize()
	if err != nil {
		return nil, err
	}

	l := &Limiter{
		maker: m,
		rate:  normalized.Rate(),
		burst: normalized.Burst,
		ttl:   normalized.TTL(),
	}

	if m.opts.prefix == "" {
		l.keyPrefix, l.sceneKey = name+":", name
	} else {
		l.keyPrefix = m.opts.prefix + ":" + name + ":"
		l.sceneKey = m.opts.prefix + ":" + name
	}

	return l, nil
}

// Close closes the maker and waits for the cleanup goroutine to exit.
func (m *Maker) Close() error {
	if m.closed.Swap(true) {
		return nil
	}

	m.cancel()
	m.wg.Wait()

	return nil
}

// cleanup evicts expired buckets periodically until the maker is closed.
func (m *Maker) cleanup() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.opts.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.evict()
		}
	}
}

// evict removes buckets that have been idle longer than their key expiration.
func (m *Maker) evict() {
	now := time.Now().UnixNano()

	for _, s := range m.shards {
		s.mu.Lock()
		for key, b := range s.buckets {
			if now-b.lastSeen.Load() > int64(b.ttl) {
				delete(s.buckets, key)
			}
		}
		s.mu.Unlock()
	}
}

// bucket returns the bucket for key, creating it when it does not exist.
func (m *Maker) bucket(key string, l *Limiter) *bucket {
	s := m.shards[shardIndex(key)]

	s.mu.RLock()
	b := s.buckets[key]
	s.mu.RUnlock()
	if b != nil {
		return b
	}

	s.mu.Lock()
	b = s.buckets[key]
	if b == nil {
		b = &bucket{
			limiter: corelimiter.NewLimiter(float64(l.burst), l.rate),
			ttl:     l.ttl,
		}
		b.lastSeen.Store(time.Now().UnixNano())
		s.buckets[key] = b
	}
	s.mu.Unlock()

	return b
}

// shard is a shard of the bucket map.
type shard struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

// bucket holds the state of one rate limiting key.
type bucket struct {
	limiter  *corelimiter.Limiter
	ttl      time.Duration
	lastSeen atomic.Int64 // Unix nanoseconds of the last access
}

// shardIndex returns the shard index of key using the FNV-1a hash.
func shardIndex(key string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)

	hash := uint32(offset32)
	for i := range len(key) {
		hash ^= uint32(key[i])
		hash *= prime32
	}

	return hash % shardCount
}
