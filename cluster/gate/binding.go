package gate

import "sync"

// bindingLocks serializes binding operations for an ID. Entries are retained only while an
// operation holds or waits for the ID's lock, so inactive IDs do not accumulate here.
type bindingLocks struct {
	mu    sync.Mutex
	users map[int64]*bindingLock
}

type bindingLock struct {
	mu   sync.Mutex
	refs int // protected by bindingLocks.mu
}

func (l *bindingLocks) lock(uid int64) func() {
	l.mu.Lock()
	if l.users == nil {
		l.users = make(map[int64]*bindingLock)
	}
	user := l.users[uid]
	if user == nil {
		user = &bindingLock{}
		l.users[uid] = user
	}
	user.refs++
	l.mu.Unlock()

	user.mu.Lock()
	return func() {
		user.mu.Unlock()
		l.mu.Lock()
		user.refs--
		if user.refs == 0 {
			delete(l.users, uid)
		}
		l.mu.Unlock()
	}
}
