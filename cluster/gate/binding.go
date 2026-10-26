package gate

import "sync"

// bindingShardNum is the number of shards used to serialize binding operations (a power of two so
// that the shard is selected with a bit operation).
const bindingShardNum = 256

// bindingLock is one shard of the binding locks.
//
// Binding operations are sharded by the low bits of the ID to dilute lock contention; the struct is
// padded to a 64-byte cache line to avoid false sharing between adjacent shards.
type bindingLock struct {
	mu sync.Mutex
	_  [56]byte
}

// bindingLocks serializes the binding operations of an ID.
//
// Operations on IDs that select different shards proceed in parallel, and no entry is allocated or
// retained per operation.
type bindingLocks struct {
	locks [bindingShardNum]bindingLock
}

// lock acquires the shard that owns id.
func (l *bindingLocks) lock(id int64) *bindingLock {
	lock := &l.locks[uint64(id)&(bindingShardNum-1)]
	lock.mu.Lock()

	return lock
}

// unlock releases the shard.
func (l *bindingLock) unlock() {
	l.mu.Unlock()
}
