package gate

import (
	"sync"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
)

func TestUnbindConnPreservesReplacement(t *testing.T) {
	s := newSession()
	first, second := newStubConn(1), newStubConn(2)
	s.addConn(first)
	s.addConn(second)
	if err := s.bind(1, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.bind(2, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.unbind(1, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("stale unbind returned %v", err)
	}
	if !s.isBound(2, 100) || s.isBound(1, 100) {
		t.Fatal("stale unbind changed the replacement binding")
	}
	if err := s.unbind(0, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("zero CID unbound a user: %v", err)
	}
	if err := s.unbind(2, 100); err != nil {
		t.Fatal(err)
	}
	if uid, err := s.uid(2); err != nil || uid != 0 {
		t.Fatalf("unbound connection has UID %d, err %v", uid, err)
	}
	if _, err := s.uid(404); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("missing connection returned %v", err)
	}
}

// TestBindAndRemConnConcurrentKeepsUserTableConsistent verifies that a bind racing with a
// connection removal never leaves a user mapping for the removed connection.
func TestBindAndRemConnConcurrentKeepsUserTableConsistent(t *testing.T) {
	for range 200 {
		s := newSession()
		conn := newStubConn(1)
		s.addConn(conn)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.bind(1, 100)
		}()
		go func() {
			defer wg.Done()
			s.remConn(conn)
		}()
		wg.Wait()

		if _, ok := s.connShard(1).conns[1]; !ok {
			if _, ok := s.userShard(100).conns[100]; ok {
				t.Fatal("removed connection left a user binding")
			}
		}
	}
}

// TestBindAndUnbindConcurrentKeepsStateConsistent verifies that a bind racing with an unbind
// leaves either a complete binding or no binding, never a mapping whose connection carries a
// different UID.
func TestBindAndUnbindConcurrentKeepsStateConsistent(t *testing.T) {
	for range 200 {
		s := newSession()
		conn := newStubConn(1)
		s.addConn(conn)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.bind(1, 100)
		}()
		go func() {
			defer wg.Done()
			_, _ = s.remove(100)
		}()
		wg.Wait()

		if bound, ok := s.userShard(100).conns[100]; ok {
			if bound != conn || conn.UID() != 100 {
				t.Fatalf("inconsistent binding: conn = %v, uid = %d", bound, conn.UID())
			}
		}
	}
}

// TestSubscribeAndUnsubscribeConcurrentKeepsStateConsistent verifies that concurrent subscription
// changes for the same connection leave the attribute and the channel table in the same state.
func TestSubscribeAndUnsubscribeConcurrentKeepsStateConsistent(t *testing.T) {
	for range 200 {
		s := newSession()
		conn := newStubConn(1)
		s.addConn(conn)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.subscribe(cluster.Conn, []int64{1}, "chat")
		}()
		go func() {
			defer wg.Done()
			_ = s.unsubscribe(cluster.Conn, []int64{1}, "chat")
		}()
		wg.Wait()

		_, inAttr := conn.Attr().Get("chat")

		s.channelsMu.RLock()
		_, inChannel := s.channels["chat"]
		s.channelsMu.RUnlock()

		if inAttr != inChannel {
			t.Fatalf("inconsistent subscription state: attr = %v, channel = %v", inAttr, inChannel)
		}
	}
}
