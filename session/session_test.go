package session

import (
	"net"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/network"
)

// mockConn 测试用连接实现
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

// mockAttr 测试用连接属性实现
type mockAttr struct {
	m map[any]any
}

func (a *mockAttr) Set(key, value any)        { a.m[key] = value }
func (a *mockAttr) Get(key any) (any, bool)   { v, ok := a.m[key]; return v, ok }
func (a *mockAttr) Del(key any) bool          { _, ok := a.m[key]; delete(a.m, key); return ok }
func (a *mockAttr) Clear()                    { a.m = make(map[any]any) }
func (a *mockAttr) Visit(fn func(key, value any) bool) {
	for k, v := range a.m {
		if !fn(k, v) {
			return
		}
	}
}

func TestRemConnRegistered(t *testing.T) {
	s := NewSession()
	conn := newMockConn(1)

	s.AddConn(conn)

	if !s.RemConn(conn) {
		t.Fatal("RemConn should return true for a registered conn")
	}

	if len(s.conns) != 0 {
		t.Fatalf("conns should be empty, got %d", len(s.conns))
	}

	// 重复移除必须返回false，防止调用方重复执行断开清理逻辑
	if s.RemConn(conn) {
		t.Fatal("RemConn should return false for an already removed conn")
	}
}

func TestRemConnUnregistered(t *testing.T) {
	s := NewSession()

	if s.RemConn(newMockConn(404)) {
		t.Fatal("RemConn should return false for an unregistered conn")
	}
}

func TestRemConnClearsUserAndChannels(t *testing.T) {
	s := NewSession()
	conn := newMockConn(1)

	s.AddConn(conn)

	if err := s.Bind(1, 100); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}

	if err := s.Subscribe(Conn, []int64{1}, "chat"); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if !s.RemConn(conn) {
		t.Fatal("RemConn should return true for a registered conn")
	}

	if len(s.conns) != 0 {
		t.Fatalf("conns should be empty, got %d", len(s.conns))
	}
	if len(s.users) != 0 {
		t.Fatalf("users should be empty, got %d", len(s.users))
	}
	if len(s.channels) != 0 {
		t.Fatalf("channels should be empty, got %d", len(s.channels))
	}
	if _, ok := conn.attr.Get("chat"); ok {
		t.Fatal("conn attrs should be cleared")
	}
}

// TestRemConnConcurrent 并发移除同一连接时仅允许一个调用返回true
func TestRemConnConcurrent(t *testing.T) {
	s := NewSession()
	conn := newMockConn(1)

	s.AddConn(conn)

	const goroutines = 32
	results := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			results <- s.RemConn(conn)
		}()
	}

	removed := 0
	for i := 0; i < goroutines; i++ {
		if <-results {
			removed++
		}
	}

	if removed != 1 {
		t.Fatalf("expect exactly one successful removal, got %d", removed)
	}
}
