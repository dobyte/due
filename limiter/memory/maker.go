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

const (
	shardCount = 32

	// evictScanBatchSize is the maximum number of buckets scanned per shard in one cleanup pass.
	evictScanBatchSize = 512
)

// Maker is a memory limiter maker.
type Maker struct {
	opts      *options
	shards    [shardCount]*shard
	scenes    map[string]uint32
	scenesMu  sync.Mutex
	nextScene uint32
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    atomic.Bool
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

	m := &Maker{}
	m.opts = o
	m.scenes = make(map[string]uint32)
	m.ctx, m.cancel = context.WithCancel(context.Background())

	for i := range shardCount {
		m.shards[i] = &shard{buckets: make(map[bucketKey]*bucket)}
	}

	m.wg.Go(m.cleanup)

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

	return &Limiter{
		maker: m,
		scene: m.sceneID(name),
		rate:  normalized.Rate(),
		burst: normalized.Burst,
		ttl:   normalized.TTL(),
	}, nil
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

// sceneID returns the stable identifier of the named scene, assigning one when it is first seen.
//
// The prefix and name are joined once here, at Make time, instead of on every Allow call.
func (m *Maker) sceneID(name string) uint32 {
	key := name
	if m.opts.prefix != "" {
		key = m.opts.prefix + ":" + name
	}

	m.scenesMu.Lock()
	id, ok := m.scenes[key]
	if !ok {
		m.nextScene++
		id = m.nextScene
		m.scenes[key] = id
	}
	m.scenesMu.Unlock()

	return id
}

// cleanup evicts expired buckets periodically until the maker is closed.
func (m *Maker) cleanup() {
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

// evict scans every shard in bounded batches.
//
// Each shard keeps a cursor over its key slice, so one pass scans at most evictScanBatchSize
// buckets per shard instead of scanning the whole map.
func (m *Maker) evict() {
	now := time.Now().UnixNano()

	for _, s := range m.shards {
		m.evictShard(s, now)
	}
}

// evictShard scans a bounded window of one shard and removes expired buckets.
//
// A deleted bucket is replaced by the last key in the slice, so the cursor can continue without an
// extra index and the slice stays compact.
func (m *Maker) evictShard(s *shard, now int64) {
	s.mu.Lock()

	if len(s.keys) == 0 {
		s.cursor = 0
		s.keys = nil
		s.mu.Unlock()
		return
	}

	if s.cursor >= len(s.keys) {
		s.cursor = 0
	}

	for processed := 0; processed < evictScanBatchSize && s.cursor < len(s.keys); processed++ {
		key := s.keys[s.cursor]
		b := s.buckets[key]

		if now-b.lastSeen.Load() <= int64(b.ttl) {
			s.cursor++
			continue
		}

		last := len(s.keys) - 1
		if s.cursor != last {
			// Move the last key into the cursor; it is processed next.
			s.keys[s.cursor] = s.keys[last]
		}

		// Clear the tail so the deleted key string is not retained by the backing array.
		s.keys[last] = bucketKey{}
		s.keys = s.keys[:last]
		delete(s.buckets, key)
	}

	if s.cursor >= len(s.keys) {
		s.cursor = 0
	}

	s.mu.Unlock()
}

// bucket returns the bucket for key, creating it when it does not exist.
func (m *Maker) bucket(key bucketKey, l *Limiter) *bucket {
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
		s.keys = append(s.keys, key)
		s.buckets[key] = b
	}
	s.mu.Unlock()

	return b
}

// shard is a shard of the bucket map.
type shard struct {
	mu      sync.RWMutex
	buckets map[bucketKey]*bucket
	keys    []bucketKey // Live keys used by cursor-based eviction.
	cursor  int         // Next key index to scan.
}

// bucket holds the state of one rate limiting key.
type bucket struct {
	limiter  *corelimiter.Limiter
	ttl      time.Duration
	lastSeen atomic.Int64 // Unix nanoseconds of the last access
}

// bucketKey identifies a bucket by scene and dimension.
//
// Using the structured key avoids building a concatenated key on every Allow call.
type bucketKey struct {
	scene     uint32
	dimension string
}

// shardIndex returns the shard index of key using the FNV-1a hash.
func shardIndex(key bucketKey) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)

	hash := uint32(offset32)
	hash ^= key.scene
	hash *= prime32

	for i := range len(key.dimension) {
		hash ^= uint32(key.dimension[i])
		hash *= prime32
	}

	return hash % shardCount
}
