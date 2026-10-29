package memory

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/limiter"
)

func newEvictMaker(t *testing.T) *Maker {
	t.Helper()

	maker := NewMaker(WithCleanupInterval(time.Hour))
	t.Cleanup(func() { _ = maker.Close() })

	return maker
}

func bucketCount(maker *Maker) int {
	total := 0

	for _, s := range maker.shards {
		s.mu.RLock()
		total += len(s.buckets)
		s.mu.RUnlock()
	}

	return total
}

func TestMaker_Evict(t *testing.T) {
	var (
		ctx   = context.Background()
		maker = newEvictMaker(t)
	)

	l, err := maker.Make("evict", limiter.Rule{Quota: 10, Period: time.Second, Burst: 10})
	if err != nil {
		t.Fatal(err)
	}

	ml := l.(*Limiter)

	for _, key := range []string{"expired", "active"} {
		if _, err = ml.Allow(ctx, key); err != nil {
			t.Fatal(err)
		}
	}

	var (
		expiredKey = bucketKey{scene: ml.scene, dimension: "expired"}
		activeKey  = bucketKey{scene: ml.scene, dimension: "active"}
	)

	expiredShard := maker.shards[shardIndex(expiredKey)]
	expiredShard.mu.RLock()
	expiredBucket := expiredShard.buckets[expiredKey]
	expiredShard.mu.RUnlock()
	if expiredBucket == nil {
		t.Fatal("expired bucket was not created")
	}

	expiredBucket.lastSeen.Store(time.Now().Add(-time.Hour).UnixNano())

	maker.evict()

	expiredShard.mu.RLock()
	_, ok := expiredShard.buckets[expiredKey]
	expiredShard.mu.RUnlock()
	if ok {
		t.Fatal("expired bucket was not removed")
	}

	activeShard := maker.shards[shardIndex(activeKey)]
	activeShard.mu.RLock()
	_, ok = activeShard.buckets[activeKey]
	activeShard.mu.RUnlock()
	if !ok {
		t.Fatal("active bucket was removed")
	}
}

func TestMaker_EvictBatch(t *testing.T) {
	var (
		ctx   = context.Background()
		maker = newEvictMaker(t)
		total = evictScanBatchSize + 50
	)

	l, err := maker.Make("evict-batch", limiter.Rule{Quota: 10, Period: time.Second, Burst: 10})
	if err != nil {
		t.Fatal(err)
	}

	for i := range total {
		if _, err = l.Allow(ctx, strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-time.Hour).UnixNano()
	for _, s := range maker.shards {
		s.mu.RLock()
		for _, b := range s.buckets {
			b.lastSeen.Store(old)
		}
		s.mu.RUnlock()
	}

	maker.evict()

	count := 0
	for _, s := range maker.shards {
		s.mu.RLock()
		count += len(s.buckets)
		s.mu.RUnlock()
	}

	if count != 0 {
		t.Fatalf("expect all buckets to be evicted, got: %d", count)
	}
}

func TestMaker_EvictCursor(t *testing.T) {
	var (
		ctx   = context.Background()
		maker = newEvictMaker(t)
		total = 20_000
	)

	l, err := maker.Make("evict-cursor", limiter.Rule{Quota: 10, Period: time.Second, Burst: 10})
	if err != nil {
		t.Fatal(err)
	}

	for i := range total {
		if _, err = l.Allow(ctx, strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-time.Hour).UnixNano()
	for _, s := range maker.shards {
		s.mu.RLock()
		for _, b := range s.buckets {
			b.lastSeen.Store(old)
		}
		s.mu.RUnlock()
	}

	maker.evict()

	remaining := bucketCount(maker)
	if remaining == 0 {
		t.Fatal("expect only a bounded batch to be evicted in one pass")
	}

	for range shardCount {
		if remaining == 0 {
			break
		}

		maker.evict()
		remaining = bucketCount(maker)
	}

	if remaining != 0 {
		t.Fatalf("expect all buckets to be evicted after cursor passes, got: %d", remaining)
	}
}

func TestMaker_EvictConcurrent(t *testing.T) {
	var (
		ctx   = context.Background()
		maker = newEvictMaker(t)
		wait  sync.WaitGroup
	)

	l, err := maker.Make("evict-concurrent", limiter.Rule{Quota: 1000, Period: time.Hour, Burst: 1000})
	if err != nil {
		t.Fatal(err)
	}

	wait.Add(2)

	go func() {
		defer wait.Done()

		for i := range 1000 {
			_, _ = l.Allow(ctx, strconv.Itoa(i))
		}
	}()

	go func() {
		defer wait.Done()

		for range 100 {
			maker.evict()
		}
	}()

	wait.Wait()
}
