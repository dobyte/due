package gate

import (
	"net"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
)

// mockConn is a connection implementation for tests.
type mockConn struct {
	id   int64
	uid  int64
	attr *mockAttr
}

func newMockConn(id int64) *mockConn {
	return &mockConn{id: id, attr: &mockAttr{m: make(map[any]any)}}
}

func (c *mockConn) ID() int64                     { return c.id }
func (c *mockConn) UID() int64                    { return c.uid }
func (c *mockConn) Attr() network.Attr            { return c.attr }
func (c *mockConn) Bind(uid int64) error          { c.uid = uid; return nil }
func (c *mockConn) Unbind() error                 { c.uid = 0; return nil }
func (c *mockConn) Push(buf buffer.Buffer) error  { buf.Release(); return nil }
func (c *mockConn) State() network.ConnState      { return 0 }
func (c *mockConn) Close(force ...bool) error     { return nil }
func (c *mockConn) LocalIP() (string, error)      { return "", nil }
func (c *mockConn) LocalAddr() (net.Addr, error)  { return nil, nil }
func (c *mockConn) RemoteIP() (string, error)     { return "", nil }
func (c *mockConn) RemoteAddr() (net.Addr, error) { return nil, nil }

// CloseIf closes the connection forcibly when it is still the connection with the given ID.
func (c *mockConn) CloseIf(id int64) error {
	if c.ID() != id {
		return errors.ErrConnectionClosed
	}

	return c.Close(true)
}

// mockAttr is a connection attribute implementation for tests.
type mockAttr struct {
	m map[any]any
}

func (a *mockAttr) Set(key, value any)      { a.m[key] = value }
func (a *mockAttr) Get(key any) (any, bool) { v, ok := a.m[key]; return v, ok }
func (a *mockAttr) Del(key any) bool        { _, ok := a.m[key]; delete(a.m, key); return ok }
func (a *mockAttr) Clear()                  { a.m = make(map[any]any) }
func (a *mockAttr) Visit(fn func(key, value any) bool) {
	for k, v := range a.m {
		if !fn(k, v) {
			return
		}
	}
}

func TestRemConnRegistered(t *testing.T) {
	s := newSession()
	conn := newMockConn(1)

	s.addConn(conn)

	if !s.remConn(conn) {
		t.Fatal("RemConn should return true for a registered conn")
	}

	if n, _ := s.stat(cluster.Conn); n != 0 {
		t.Fatalf("conns should be empty, got %d", n)
	}

	// A repeated removal must return false so that the caller does not run its disconnect cleanup
	// logic twice.
	if s.remConn(conn) {
		t.Fatal("RemConn should return false for an already removed conn")
	}
}

func TestRemConnUnregistered(t *testing.T) {
	s := newSession()

	if s.remConn(newMockConn(404)) {
		t.Fatal("RemConn should return false for an unregistered conn")
	}
}

func TestRemConnClearsUserAndChannels(t *testing.T) {
	s := newSession()
	conn := newMockConn(1)

	s.addConn(conn)

	if err := s.bind(1, 100); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}

	if err := s.subscribe(cluster.Conn, []int64{1}, "chat"); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if !s.remConn(conn) {
		t.Fatal("RemConn should return true for a registered conn")
	}

	if n, _ := s.stat(cluster.Conn); n != 0 {
		t.Fatalf("conns should be empty, got %d", n)
	}
	if n, _ := s.stat(cluster.User); n != 0 {
		t.Fatalf("users should be empty, got %d", n)
	}
	if channelCount(s) != 0 {
		t.Fatalf("channels should be empty, got %d", channelCount(s))
	}
	if _, ok := conn.attr.Get("chat"); ok {
		t.Fatal("conn attrs should be cleared")
	}
}

// TestRemConnConcurrent verifies that concurrent removals of the same connection allow exactly one
// call to report true.
func TestRemConnConcurrent(t *testing.T) {
	s := newSession()
	conn := newMockConn(1)

	s.addConn(conn)

	const goroutines = 32
	results := make(chan bool, goroutines)

	for range goroutines {
		go func() {
			results <- s.remConn(conn)
		}()
	}

	removed := 0
	for range goroutines {
		if <-results {
			removed++
		}
	}

	if removed != 1 {
		t.Fatalf("expect exactly one successful removal, got %d", removed)
	}
}

// channelCount returns the number of channels in the session table.
func channelCount(s *session) int {
	total := 0
	for i := range s.channels {
		s.channels[i].rw.RLock()
		total += len(s.channels[i].channels)
		s.channels[i].rw.RUnlock()
	}
	return total
}

// channelSubscriberCount returns the number of subscribers of channel.
func channelSubscriberCount(s *session, channel string) int {
	subscribers := s.loadChannel(channel)
	if subscribers == nil {
		return 0
	}

	subscribers.mu.Lock()
	count := len(subscribers.members)
	subscribers.mu.Unlock()

	return count
}
