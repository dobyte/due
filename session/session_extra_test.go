package session

import (
	"net"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
)

// stubAttr is a minimal network.Attr implementation for the session tests.
type stubAttr struct {
	m map[any]any
}

// newStubAttr creates an empty attribute store.
func newStubAttr() *stubAttr {
	return &stubAttr{m: make(map[any]any)}
}

func (a *stubAttr) Set(key, value any)      { a.m[key] = value }
func (a *stubAttr) Get(key any) (any, bool) { v, ok := a.m[key]; return v, ok }
func (a *stubAttr) Del(key any) bool        { _, ok := a.m[key]; delete(a.m, key); return ok }
func (a *stubAttr) Clear()                  { a.m = make(map[any]any) }

func (a *stubAttr) Visit(fn func(key, value any) bool) {
	for k, v := range a.m {
		if !fn(k, v) {
			return
		}
	}
}

// stubConn is a configurable network.Conn implementation for the session tests.
type stubConn struct {
	id         int64
	uid        int64
	attr       network.Attr
	bindErr    error
	unbindErr  error
	pushErr    error
	closeErr   error
	localIP    string
	localAddr  net.Addr
	remoteIP   string
	remoteAddr net.Addr
	pushes     int
	closes     int
}

// newStubConn creates a stub connection with the given ID and a fresh attribute store.
func newStubConn(id int64) *stubConn {
	return &stubConn{id: id, attr: newStubAttr()}
}

func (c *stubConn) ID() int64          { return c.id }
func (c *stubConn) UID() int64         { return c.uid }
func (c *stubConn) Attr() network.Attr { return c.attr }

func (c *stubConn) Bind(uid int64) error {
	if c.bindErr != nil {
		return c.bindErr
	}
	c.uid = uid
	return nil
}

func (c *stubConn) Unbind() error {
	if c.unbindErr != nil {
		return c.unbindErr
	}
	c.uid = 0
	return nil
}

func (c *stubConn) Push(buf buffer.Buffer) error {
	if c.pushErr != nil {
		return c.pushErr
	}
	c.pushes++
	buf.Release()
	return nil
}

func (c *stubConn) State() network.ConnState      { return network.ConnOpened }
func (c *stubConn) Close(force ...bool) error     { c.closes++; return c.closeErr }
func (c *stubConn) LocalIP() (string, error)      { return c.localIP, nil }
func (c *stubConn) LocalAddr() (net.Addr, error)  { return c.localAddr, nil }
func (c *stubConn) RemoteIP() (string, error)     { return c.remoteIP, nil }
func (c *stubConn) RemoteAddr() (net.Addr, error) { return c.remoteAddr, nil }

// newSessionWithUser creates a session holding a single connection bound to uid.
func newSessionWithUser(cid, uid int64) (*Session, *stubConn) {
	s := NewSession()
	conn := newStubConn(cid)
	s.AddConn(conn)
	_ = s.Bind(cid, uid)
	return s, conn
}

// TestKindStringExtra verifies String for every kind, including unknown values.
func TestKindStringExtra(t *testing.T) {
	cases := []struct {
		name string
		kind Kind
		want string
	}{
		{"conn", Conn, "conn"},
		{"user", User, "user"},
		{"zero", Kind(0), ""},
		{"unknown", Kind(99), ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.kind.String(); got != c.want {
				t.Errorf("Kind.String() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestHasExtra verifies Has for present, missing and unknown session kinds.
func TestHasExtra(t *testing.T) {
	s, _ := newSessionWithUser(1, 100)

	cases := []struct {
		name        string
		kind        Kind
		target      int64
		wantOK      bool
		wantInvalid bool
	}{
		{"conn present", Conn, 1, true, false},
		{"conn missing", Conn, 2, false, false},
		{"user present", User, 100, true, false},
		{"user missing", User, 200, false, false},
		{"invalid kind", Kind(0), 1, false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := s.Has(c.kind, c.target)
			if ok != c.wantOK {
				t.Errorf("Has() ok = %v, want %v", ok, c.wantOK)
			}
			if c.wantInvalid {
				if err != errors.ErrInvalidSessionKind {
					t.Errorf("Has() error = %v, want %v", err, errors.ErrInvalidSessionKind)
				}
			} else if err != nil {
				t.Errorf("Has() error = %v, want nil", err)
			}
		})
	}
}

// TestBindExtra verifies the binding and replacement rules of Bind.
func TestBindExtra(t *testing.T) {
	t.Run("conn not found", func(t *testing.T) {
		s := NewSession()
		if err := s.Bind(1, 100); err != errors.ErrNotFoundSession {
			t.Errorf("Bind() error = %v, want %v", err, errors.ErrNotFoundSession)
		}
	})

	t.Run("bind success", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)

		if err := s.Bind(1, 100); err != nil {
			t.Fatalf("Bind() error = %v, want nil", err)
		}
		if conn.UID() != 100 {
			t.Errorf("conn uid = %d, want 100", conn.UID())
		}
		if s.users[100] != conn {
			t.Error("the conn must be registered under the new uid")
		}
	})

	t.Run("rebind same uid is a no-op", func(t *testing.T) {
		s, conn := newSessionWithUser(1, 100)

		if err := s.Bind(1, 100); err != nil {
			t.Fatalf("Bind() error = %v, want nil", err)
		}
		if s.users[100] != conn {
			t.Error("the binding must be unchanged")
		}
	})

	t.Run("rebind different uid drops the old binding", func(t *testing.T) {
		s, conn := newSessionWithUser(1, 100)

		if err := s.Bind(1, 200); err != nil {
			t.Fatalf("Bind() error = %v, want nil", err)
		}
		if _, ok := s.users[100]; ok {
			t.Error("the old uid binding must be dropped")
		}
		if s.users[200] != conn {
			t.Error("the conn must be registered under the new uid")
		}
	})

	t.Run("replacing a binding closes the old conn", func(t *testing.T) {
		s := NewSession()
		old := newStubConn(1)
		replacement := newStubConn(2)
		s.AddConn(old)
		s.AddConn(replacement)

		_ = s.Bind(1, 100)
		if err := s.Bind(2, 100); err != nil {
			t.Fatalf("Bind() error = %v, want nil", err)
		}
		if old.closes != 1 {
			t.Errorf("old conn close count = %d, want 1", old.closes)
		}
		if s.users[100] != replacement {
			t.Error("the replacement conn must own the uid")
		}
	})

	t.Run("close error while replacing is tolerated", func(t *testing.T) {
		s := NewSession()
		old := newStubConn(1)
		old.closeErr = errors.New("close failed")
		replacement := newStubConn(2)
		s.AddConn(old)
		s.AddConn(replacement)

		_ = s.Bind(1, 100)
		if err := s.Bind(2, 100); err != nil {
			t.Errorf("Bind() error = %v, want nil", err)
		}
		if old.closes != 1 {
			t.Errorf("old conn close count = %d, want 1", old.closes)
		}
	})

	t.Run("conn bind failure is reported", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		want := errors.New("bind failed")
		conn.bindErr = want
		s.AddConn(conn)

		if err := s.Bind(1, 100); err != want {
			t.Errorf("Bind() error = %v, want %v", err, want)
		}
	})

	t.Run("rebind failure keeps the old binding", func(t *testing.T) {
		s, _ := newSessionWithUser(1, 100)
		conn := s.conns[1]
		conn.(*stubConn).bindErr = errors.New("bind failed")

		if err := s.Bind(1, 200); err == nil {
			t.Error("Bind() must report the underlying bind failure")
		}
		if _, ok := s.users[100]; !ok {
			t.Error("the old binding must be kept when the rebind fails")
		}
	})
}

// TestUnbindExtra verifies Unbind for success, missing users and failing unbind calls.
func TestUnbindExtra(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		s := NewSession()
		if _, err := s.Unbind(100); err != errors.ErrNotFoundSession {
			t.Errorf("Unbind() error = %v, want %v", err, errors.ErrNotFoundSession)
		}
	})

	t.Run("success", func(t *testing.T) {
		s, conn := newSessionWithUser(1, 100)

		cid, err := s.Unbind(100)
		if err != nil {
			t.Fatalf("Unbind() error = %v, want nil", err)
		}
		if cid != 1 {
			t.Errorf("Unbind() conn id = %d, want 1", cid)
		}
		if conn.UID() != 0 {
			t.Errorf("conn uid = %d, want 0", conn.UID())
		}
		if _, ok := s.users[100]; ok {
			t.Error("the user binding must be removed")
		}
	})

	t.Run("unbind error is tolerated", func(t *testing.T) {
		s, conn := newSessionWithUser(1, 100)
		conn.unbindErr = errors.New("unbind failed")

		cid, err := s.Unbind(100)
		if err != nil {
			t.Fatalf("Unbind() error = %v, want nil", err)
		}
		if cid != 1 {
			t.Errorf("Unbind() conn id = %d, want 1", cid)
		}
	})
}

// locationCall wraps a location accessor so that it can be exercised uniformly.
type locationCall struct {
	name string
	call func(s *Session, kind Kind, target int64) (any, error)
}

// TestLocationAccessors verifies the local and remote address accessors.
func TestLocationAccessors(t *testing.T) {
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	s := NewSession()
	conn := newStubConn(1)
	conn.localIP = "127.0.0.1"
	conn.localAddr = addr
	conn.remoteIP = "10.0.0.1"
	conn.remoteAddr = addr
	s.AddConn(conn)
	if err := s.Bind(1, 100); err != nil {
		t.Fatalf("Bind() error = %v, want nil", err)
	}

	calls := []locationCall{
		{"LocalIP", func(s *Session, kind Kind, target int64) (any, error) { return s.LocalIP(kind, target) }},
		{"LocalAddr", func(s *Session, kind Kind, target int64) (any, error) { return s.LocalAddr(kind, target) }},
		{"RemoteIP", func(s *Session, kind Kind, target int64) (any, error) { return s.RemoteIP(kind, target) }},
		{"RemoteAddr", func(s *Session, kind Kind, target int64) (any, error) { return s.RemoteAddr(kind, target) }},
	}

	for _, c := range calls {
		t.Run(c.name+"/conn", func(t *testing.T) {
			v, err := c.call(s, Conn, 1)
			if err != nil {
				t.Errorf("%s(Conn) error = %v, want nil", c.name, err)
			}
			if v == nil {
				t.Errorf("%s(Conn) value = nil, want non-nil", c.name)
			}
		})

		t.Run(c.name+"/user", func(t *testing.T) {
			v, err := c.call(s, User, 100)
			if err != nil {
				t.Errorf("%s(User) error = %v, want nil", c.name, err)
			}
			if v == nil {
				t.Errorf("%s(User) value = nil, want non-nil", c.name)
			}
		})

		t.Run(c.name+"/missing", func(t *testing.T) {
			if _, err := c.call(s, Conn, 999); err != errors.ErrNotFoundSession {
				t.Errorf("%s(Conn) error = %v, want %v", c.name, err, errors.ErrNotFoundSession)
			}
		})

		t.Run(c.name+"/invalid kind", func(t *testing.T) {
			if _, err := c.call(s, Kind(0), 1); err != errors.ErrInvalidSessionKind {
				t.Errorf("%s error = %v, want %v", c.name, err, errors.ErrInvalidSessionKind)
			}
		})
	}
}

// TestCloseExtra verifies Close for found, missing and failing sessions.
func TestCloseExtra(t *testing.T) {
	s := NewSession()
	conn := newStubConn(1)
	s.AddConn(conn)

	if err := s.Close(Conn, 1); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	if conn.closes != 1 {
		t.Errorf("conn close count = %d, want 1", conn.closes)
	}

	if err := s.Close(Conn, 2); err != errors.ErrNotFoundSession {
		t.Errorf("Close() error = %v, want %v", err, errors.ErrNotFoundSession)
	}

	if err := s.Close(Kind(0), 1); err != errors.ErrInvalidSessionKind {
		t.Errorf("Close() error = %v, want %v", err, errors.ErrInvalidSessionKind)
	}

	conn.closeErr = errors.New("close failed")
	if err := s.Close(Conn, 1); err == nil {
		t.Error("Close() must report the underlying close failure")
	}
}

// TestPushExtra verifies Push for missing sessions, failures and the disconnect flag.
func TestPushExtra(t *testing.T) {
	t.Run("conn not found releases buffer", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		if err := s.Push(Conn, 1, false, buf); err != errors.ErrNotFoundSession {
			t.Errorf("Push() error = %v, want %v", err, errors.ErrNotFoundSession)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("push success", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)

		if err := s.Push(Conn, 1, false, buffer.NewBytes([]byte("data"))); err != nil {
			t.Errorf("Push() error = %v, want nil", err)
		}
		if conn.pushes != 1 {
			t.Errorf("conn push count = %d, want 1", conn.pushes)
		}
	})

	t.Run("push failure releases buffer", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		want := errors.New("push failed")
		conn.pushErr = want
		s.AddConn(conn)

		buf := buffer.NewBytes([]byte("data"))
		if err := s.Push(Conn, 1, false, buf); err != want {
			t.Errorf("Push() error = %v, want %v", err, want)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("push with disconnect closes conn", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)

		if err := s.Push(Conn, 1, true, buffer.NewBytes([]byte("data"))); err != nil {
			t.Errorf("Push() error = %v, want nil", err)
		}
		if conn.closes != 1 {
			t.Errorf("conn close count = %d, want 1", conn.closes)
		}
	})

	t.Run("push disconnect close error is tolerated", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		conn.closeErr = errors.New("close failed")
		s.AddConn(conn)

		if err := s.Push(Conn, 1, true, buffer.NewBytes([]byte("data"))); err != nil {
			t.Errorf("Push() error = %v, want nil", err)
		}
	})

	t.Run("push by user", func(t *testing.T) {
		s, conn := newSessionWithUser(1, 100)

		if err := s.Push(User, 100, false, buffer.NewBytes([]byte("data"))); err != nil {
			t.Errorf("Push() error = %v, want nil", err)
		}
		if conn.pushes != 1 {
			t.Errorf("conn push count = %d, want 1", conn.pushes)
		}
	})
}

// TestMulticastExtra verifies Multicast for empty targets, unknown kinds and batch pushes.
func TestMulticastExtra(t *testing.T) {
	t.Run("empty targets", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		n, err := s.Multicast(Conn, nil, false, buf)
		if n != 0 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (0, nil)", n, err)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("invalid kind", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		n, err := s.Multicast(Kind(0), []int64{1}, false, buf)
		if n != 0 || err != errors.ErrInvalidSessionKind {
			t.Errorf("Multicast() = (%d, %v), want (0, %v)", n, err, errors.ErrInvalidSessionKind)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("no matching conns", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		n, err := s.Multicast(Conn, []int64{1, 2}, false, buf)
		if n != 0 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (0, nil)", n, err)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("single conn", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)

		n, err := s.Multicast(Conn, []int64{1, 2}, false, buffer.NewBytes([]byte("data")))
		if n != 1 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (1, nil)", n, err)
		}
		if conn.pushes != 1 {
			t.Errorf("conn push count = %d, want 1", conn.pushes)
		}
	})

	t.Run("single conn push failure", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		conn.pushErr = errors.New("push failed")
		s.AddConn(conn)

		buf := buffer.NewBytes([]byte("data"))
		n, err := s.Multicast(Conn, []int64{1, 2}, false, buf)
		if n != 0 || err == nil {
			t.Errorf("Multicast() = (%d, %v), want (0, non-nil)", n, err)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("single conn with disconnect", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)

		n, err := s.Multicast(Conn, []int64{1}, true, buffer.NewBytes([]byte("data")))
		if n != 1 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (1, nil)", n, err)
		}
		if conn.closes != 1 {
			t.Errorf("conn close count = %d, want 1", conn.closes)
		}
	})

	t.Run("multiple conns", func(t *testing.T) {
		s := NewSession()
		c1, c2, c3 := newStubConn(1), newStubConn(2), newStubConn(3)
		s.AddConn(c1)
		s.AddConn(c2)
		s.AddConn(c3)

		n, err := s.Multicast(Conn, []int64{1, 2, 3}, false, buffer.NewBytes([]byte("data")))
		if n != 3 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (3, nil)", n, err)
		}
	})

	t.Run("multiple conns with disconnect", func(t *testing.T) {
		s := NewSession()
		c1, c2 := newStubConn(1), newStubConn(2)
		c2.closeErr = errors.New("close failed")
		s.AddConn(c1)
		s.AddConn(c2)

		n, err := s.Multicast(Conn, []int64{1, 2}, true, buffer.NewBytes([]byte("data")))
		if n != 2 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (2, nil)", n, err)
		}
		if c1.closes != 1 || c2.closes != 1 {
			t.Errorf("close counts = (%d, %d), want (1, 1)", c1.closes, c2.closes)
		}
	})

	t.Run("multiple conns partial failure", func(t *testing.T) {
		s := NewSession()
		c1, c2 := newStubConn(1), newStubConn(2)
		c2.pushErr = errors.New("push failed")
		s.AddConn(c1)
		s.AddConn(c2)

		n, err := s.Multicast(Conn, []int64{1, 2}, false, buffer.NewBytes([]byte("data")))
		if n != 1 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (1, nil)", n, err)
		}
	})

	t.Run("multiple conns total failure", func(t *testing.T) {
		s := NewSession()
		c1, c2 := newStubConn(1), newStubConn(2)
		c1.pushErr = errors.New("push failed")
		c2.pushErr = errors.New("push failed")
		s.AddConn(c1)
		s.AddConn(c2)

		n, err := s.Multicast(Conn, []int64{1, 2}, false, buffer.NewBytes([]byte("data")))
		if n != 0 || err == nil {
			t.Errorf("Multicast() = (%d, %v), want (0, non-nil)", n, err)
		}
	})

	t.Run("by user", func(t *testing.T) {
		s := NewSession()
		c1, c2 := newStubConn(1), newStubConn(2)
		s.AddConn(c1)
		s.AddConn(c2)
		_ = s.Bind(1, 100)
		_ = s.Bind(2, 200)

		n, err := s.Multicast(User, []int64{100, 200}, false, buffer.NewBytes([]byte("data")))
		if n != 2 || err != nil {
			t.Errorf("Multicast() = (%d, %v), want (2, nil)", n, err)
		}
	})
}

// TestBroadcastExtra verifies Broadcast for unknown kinds, empty sessions and batch pushes.
func TestBroadcastExtra(t *testing.T) {
	t.Run("invalid kind", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		n, err := s.Broadcast(Kind(0), false, buf)
		if n != 0 || err != errors.ErrInvalidSessionKind {
			t.Errorf("Broadcast() = (%d, %v), want (0, %v)", n, err, errors.ErrInvalidSessionKind)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("no conns", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		n, err := s.Broadcast(Conn, false, buf)
		if n != 0 || err != nil {
			t.Errorf("Broadcast() = (%d, %v), want (0, nil)", n, err)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("single conn", func(t *testing.T) {
		s := NewSession()
		s.AddConn(newStubConn(1))

		n, err := s.Broadcast(Conn, false, buffer.NewBytes([]byte("data")))
		if n != 1 || err != nil {
			t.Errorf("Broadcast() = (%d, %v), want (1, nil)", n, err)
		}
	})

	t.Run("multiple conns", func(t *testing.T) {
		s := NewSession()
		s.AddConn(newStubConn(1))
		s.AddConn(newStubConn(2))

		n, err := s.Broadcast(Conn, false, buffer.NewBytes([]byte("data")))
		if n != 2 || err != nil {
			t.Errorf("Broadcast() = (%d, %v), want (2, nil)", n, err)
		}
	})

	t.Run("by user", func(t *testing.T) {
		s := NewSession()
		c1, c2 := newStubConn(1), newStubConn(2)
		s.AddConn(c1)
		s.AddConn(c2)
		_ = s.Bind(1, 100)
		_ = s.Bind(2, 200)

		n, err := s.Broadcast(User, false, buffer.NewBytes([]byte("data")))
		if n != 2 || err != nil {
			t.Errorf("Broadcast() = (%d, %v), want (2, nil)", n, err)
		}
	})
}

// TestPublishExtra verifies Publish for missing, single and multiple subscribers.
func TestPublishExtra(t *testing.T) {
	t.Run("no subscribers", func(t *testing.T) {
		s := NewSession()
		buf := buffer.NewBytes([]byte("data"))

		n, err := s.Publish("chat", false, buf)
		if n != 0 || err != nil {
			t.Errorf("Publish() = (%d, %v), want (0, nil)", n, err)
		}
		if buf.Len() != 0 {
			t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
		}
	})

	t.Run("single subscriber", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)
		if err := s.Subscribe(Conn, []int64{1}, "chat"); err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}

		n, err := s.Publish("chat", false, buffer.NewBytes([]byte("data")))
		if n != 1 || err != nil {
			t.Errorf("Publish() = (%d, %v), want (1, nil)", n, err)
		}
		if conn.pushes != 1 {
			t.Errorf("conn push count = %d, want 1", conn.pushes)
		}
	})

	t.Run("multiple subscribers", func(t *testing.T) {
		s := NewSession()
		s.AddConn(newStubConn(1))
		s.AddConn(newStubConn(2))
		s.AddConn(newStubConn(3))
		if err := s.Subscribe(Conn, []int64{1, 2, 3}, "chat"); err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}

		n, err := s.Publish("chat", false, buffer.NewBytes([]byte("data")))
		if n != 3 || err != nil {
			t.Errorf("Publish() = (%d, %v), want (3, nil)", n, err)
		}
	})
}

// TestSubscribeExtra verifies Subscribe for empty targets, unknown kinds and channel creation.
func TestSubscribeExtra(t *testing.T) {
	t.Run("empty targets", func(t *testing.T) {
		s := NewSession()
		if err := s.Subscribe(Conn, nil, "chat"); err != nil {
			t.Errorf("Subscribe() error = %v, want nil", err)
		}
	})

	t.Run("invalid kind", func(t *testing.T) {
		s := NewSession()
		if err := s.Subscribe(Kind(0), []int64{1}, "chat"); err != errors.ErrInvalidSessionKind {
			t.Errorf("Subscribe() error = %v, want %v", err, errors.ErrInvalidSessionKind)
		}
	})

	t.Run("missing target is skipped", func(t *testing.T) {
		s := NewSession()
		if err := s.Subscribe(Conn, []int64{99}, "chat"); err != nil {
			t.Errorf("Subscribe() error = %v, want nil", err)
		}
		if len(s.channels) != 0 {
			t.Errorf("channel count = %d, want 0", len(s.channels))
		}
	})

	t.Run("existing and new channels", func(t *testing.T) {
		s := NewSession()
		c1, c2 := newStubConn(1), newStubConn(2)
		s.AddConn(c1)
		s.AddConn(c2)

		if err := s.Subscribe(Conn, []int64{1}, "chat"); err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}
		if err := s.Subscribe(Conn, []int64{2}, "chat"); err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}
		if len(s.channels["chat"]) != 2 {
			t.Errorf("subscriber count = %d, want 2", len(s.channels["chat"]))
		}
		if _, ok := c1.Attr().Get("chat"); !ok {
			t.Error("the subscription must be recorded as a conn attribute")
		}
	})

	t.Run("by user", func(t *testing.T) {
		s, _ := newSessionWithUser(1, 100)

		if err := s.Subscribe(User, []int64{100}, "chat"); err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}
		if len(s.channels["chat"]) != 1 {
			t.Errorf("subscriber count = %d, want 1", len(s.channels["chat"]))
		}
	})
}

// TestUnsubscribeExtra verifies Unsubscribe for empty targets, unknown kinds and removal rules.
func TestUnsubscribeExtra(t *testing.T) {
	t.Run("empty targets", func(t *testing.T) {
		s := NewSession()
		if err := s.Unsubscribe(Conn, nil, "chat"); err != nil {
			t.Errorf("Unsubscribe() error = %v, want nil", err)
		}
	})

	t.Run("invalid kind", func(t *testing.T) {
		s := NewSession()
		if err := s.Unsubscribe(Kind(0), []int64{1}, "chat"); err != errors.ErrInvalidSessionKind {
			t.Errorf("Unsubscribe() error = %v, want %v", err, errors.ErrInvalidSessionKind)
		}
	})

	t.Run("missing target is skipped", func(t *testing.T) {
		s := NewSession()
		if err := s.Unsubscribe(Conn, []int64{99}, "chat"); err != nil {
			t.Errorf("Unsubscribe() error = %v, want nil", err)
		}
	})

	t.Run("not subscribed is skipped", func(t *testing.T) {
		s := NewSession()
		s.AddConn(newStubConn(1))

		if err := s.Unsubscribe(Conn, []int64{1}, "chat"); err != nil {
			t.Errorf("Unsubscribe() error = %v, want nil", err)
		}
		if len(s.channels) != 0 {
			t.Errorf("channel count = %d, want 0", len(s.channels))
		}
	})

	t.Run("removes the last subscriber and the channel", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)
		_ = s.Subscribe(Conn, []int64{1}, "chat")

		if err := s.Unsubscribe(Conn, []int64{1}, "chat"); err != nil {
			t.Fatalf("Unsubscribe() error = %v, want nil", err)
		}
		if len(s.channels) != 0 {
			t.Errorf("channel count = %d, want 0", len(s.channels))
		}
		if _, ok := conn.Attr().Get("chat"); ok {
			t.Error("the subscription attribute must be removed")
		}
	})

	t.Run("keeps the channel with remaining subscribers", func(t *testing.T) {
		s := NewSession()
		s.AddConn(newStubConn(1))
		s.AddConn(newStubConn(2))
		_ = s.Subscribe(Conn, []int64{1, 2}, "chat")

		if err := s.Unsubscribe(Conn, []int64{1}, "chat"); err != nil {
			t.Fatalf("Unsubscribe() error = %v, want nil", err)
		}
		if len(s.channels["chat"]) != 1 {
			t.Errorf("subscriber count = %d, want 1", len(s.channels["chat"]))
		}
	})

	t.Run("by user", func(t *testing.T) {
		s, _ := newSessionWithUser(1, 100)
		_ = s.Subscribe(User, []int64{100}, "chat")

		if err := s.Unsubscribe(User, []int64{100}, "chat"); err != nil {
			t.Fatalf("Unsubscribe() error = %v, want nil", err)
		}
		if len(s.channels) != 0 {
			t.Errorf("channel count = %d, want 0", len(s.channels))
		}
	})
}

// TestStatExtra verifies Stat for every kind.
func TestStatExtra(t *testing.T) {
	s := NewSession()

	if n, err := s.Stat(Conn); n != 0 || err != nil {
		t.Errorf("Stat(Conn) = (%d, %v), want (0, nil)", n, err)
	}
	if _, err := s.Stat(Kind(0)); err != errors.ErrInvalidSessionKind {
		t.Errorf("Stat() error = %v, want %v", err, errors.ErrInvalidSessionKind)
	}

	c1, c2, c3 := newStubConn(1), newStubConn(2), newStubConn(3)
	s.AddConn(c1)
	s.AddConn(c2)
	s.AddConn(c3)
	_ = s.Bind(1, 100)
	_ = s.Bind(2, 200)

	if n, err := s.Stat(Conn); n != 3 || err != nil {
		t.Errorf("Stat(Conn) = (%d, %v), want (3, nil)", n, err)
	}
	if n, err := s.Stat(User); n != 2 || err != nil {
		t.Errorf("Stat(User) = (%d, %v), want (2, nil)", n, err)
	}
}

// TestDoBatchPushEmpty verifies the defensive empty-input branch of doBatchPush, which is not
// reachable through the exported batch helpers because they all return early for an empty set.
func TestDoBatchPushEmpty(t *testing.T) {
	s := NewSession()
	buf := buffer.NewBytes([]byte("data"))

	n, err := s.doBatchPush(nil, false, buf)
	if n != 0 || err != nil {
		t.Errorf("doBatchPush() = (%d, %v), want (0, nil)", n, err)
	}
	if buf.Len() != 0 {
		t.Errorf("buffer len = %d, want 0 (released)", buf.Len())
	}
}

// TestRemConnExtra verifies RemConn when the connection has no attributes and when an attribute
// key is not a channel name.
func TestRemConnExtra(t *testing.T) {
	t.Run("nil attr", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		conn.attr = nil
		s.AddConn(conn)

		if !s.RemConn(conn) {
			t.Error("RemConn() = false, want true")
		}
	})

	t.Run("non-string attr key", func(t *testing.T) {
		s := NewSession()
		conn := newStubConn(1)
		s.AddConn(conn)
		conn.Attr().Set(123, struct{}{})

		if !s.RemConn(conn) {
			t.Error("RemConn() = false, want true")
		}
		if _, ok := conn.Attr().Get(123); ok {
			t.Error("the connection attributes must be cleared")
		}
	})
}
