// Package lock provides a distributed lock abstraction built on a [Maker]/[Locker] pair.
//
// A [Maker] creates [Locker] instances by name, and a [Locker] acquires and releases a lock. The
// package holds a global maker so that lockers can be created through [Make] without passing the
// maker around explicitly.
package lock

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/log"
)

var globalMaker Maker

// Maker creates and closes lockers.
type Maker interface {
	// Make creates a locker with the given name.
	Make(name string) Locker
	// Close closes the maker.
	Close() error
}

// Option configures how a lock is acquired.
type Option struct {
	Once       bool          // Once reports whether the lock is acquired only once; by default it is acquired in blocking mode until it succeeds.
	Expiration time.Duration // Expiration is the lock expiration time.
}

// Locker is a distributed lock that can be acquired and released.
type Locker interface {
	// Acquire acquires the lock.
	Acquire(ctx context.Context) error
	// TryAcquire tries to acquire the lock with an optional expiration.
	TryAcquire(ctx context.Context, expiration ...time.Duration) error
	// Release releases the lock.
	Release(ctx context.Context) error
}

// SetMaker sets the global locker maker. It closes the previous maker, if any, before replacing it.
func SetMaker(maker Maker) {
	if maker == nil {
		log.Warn("cannot set a nil lock-maker")
		return
	}

	if globalMaker != nil {
		if err := globalMaker.Close(); err != nil {
			log.Error("close lock-maker failed: %v", err)
		}
	}

	globalMaker = maker
}

// GetMaker returns the global locker maker.
func GetMaker() Maker {
	return globalMaker
}

// Make creates a locker with the given name through the global maker. It returns nil when no maker
// has been set.
func Make(name string) Locker {
	if globalMaker != nil {
		return globalMaker.Make(name)
	} else {
		return nil
	}
}

// Close closes the global maker. It does nothing when no maker has been set.
func Close() error {
	if globalMaker != nil {
		return globalMaker.Close()
	} else {
		return nil
	}
}
