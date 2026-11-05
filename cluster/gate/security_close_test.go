package gate

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/task"
)

// securityCloseConn models a transport whose Close waits for its receive callback to return.
type securityCloseConn struct {
	*securityConn
	calls        atomic.Int64
	force        atomic.Bool
	readerExited <-chan struct{}
	finished     chan struct{}
}

func (c *securityCloseConn) Close(force ...bool) error {
	c.calls.Add(1)
	c.force.Store(len(force) > 0 && force[0])
	if c.readerExited != nil {
		<-c.readerExited
	}
	c.closed.Store(true)
	if c.finished != nil {
		c.finished <- struct{}{}
	}
	return nil
}

// CloseIf keeps the compare-and-close behavior of the security close path.
func (c *securityCloseConn) CloseIf(id int64) error {
	if c.ID() != id {
		return errors.ErrConnectionClosed
	}

	return c.Close(true)
}

// reusedSecurityConn records the ID passed to CloseIf so the test can verify that the close uses
// the ID captured with the security state instead of the ID the object carries now.
type reusedSecurityConn struct {
	*securityConn
	received atomic.Int64
}

func (c *reusedSecurityConn) CloseIf(id int64) error {
	c.received.Store(id)

	if c.ID() != id {
		return errors.ErrConnectionClosed
	}

	return c.Close(true)
}

// TestSecurityCloseUsesCapturedConnectionID verifies that a stale security state does not close a
// connection object that has been recycled and reused by a newer connection.
func TestSecurityCloseUsesCapturedConnectionID(t *testing.T) {
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("reuse-test")}))
	conn := &reusedSecurityConn{securityConn: &securityConn{id: 1}}
	g.security.wrap(conn)
	t.Cleanup(func() { g.security.remove(1) })

	// Simulate the transport recycling the object for a newer connection.
	conn.id = 2

	g.security.closeConn(1)
	g.security.closers.Wait()

	if got := conn.received.Load(); got != 1 {
		t.Fatalf("close id = %d, want the captured id 1", got)
	}
	if conn.closed.Load() {
		t.Fatal("the reused connection must not be closed by the stale security state")
	}
}

func TestSecurityCloseFromReceiveDoesNotSubmitToTaskPool(t *testing.T) {
	// SetPool replaces and releases process-wide state. Keep this test and other task-pool users
	// serial; the cleanup installs a fresh pool because the previous one has been released.
	pool := task.NewPool(task.WithSize(1), task.WithNonblocking(false))
	task.SetPool(pool)
	busy, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		task.SetPool(task.NewPool())
	})
	if err := pool.AddTask(func() { close(busy); <-release }); err != nil {
		t.Fatal(err)
	}
	<-busy
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("close-receive-test")}))
	readerExited := make(chan struct{})
	conn := &securityCloseConn{securityConn: &securityConn{id: 1}, readerExited: readerExited,
		finished: make(chan struct{}, 1)}
	g.security.wrap(conn)
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		g.security.remove(1)
		g.security.closers.Wait()
	})
	buf, err := packet.PackMessage(1, 1, []byte("before handshake"))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		g.handleReceive(conn, buf)
		close(readerExited)
	}()
	select {
	case <-conn.finished:
	case <-time.After(time.Second):
		t.Fatal("security close blocked on the reader or global task pool")
	}
	g.security.closers.Wait()
	if !conn.force.Load() {
		t.Fatal("security rejection must force close")
	}
}

func TestSecurityCloseOnlyOnce(t *testing.T) {
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("close-once-test")}))
	conn := &securityCloseConn{securityConn: &securityConn{id: 1}}
	g.security.wrap(conn)
	t.Cleanup(func() { g.security.remove(1) })
	var requests sync.WaitGroup
	for range 32 {
		requests.Go(func() { g.security.closeConn(1) })
	}
	requests.Wait()
	g.security.closers.Wait()
	if got := conn.calls.Load(); got != 1 {
		t.Fatalf("close calls = %d, want 1", got)
	}
	if !conn.force.Load() {
		t.Fatal("security rejection must force close")
	}
	value, _ := g.security.states.Load(int64(1))
	state := value.(*securityState)
	state.mu.Lock()
	stopped := state.closed && state.timer == nil && state.pending == nil
	state.mu.Unlock()
	if !stopped {
		t.Fatal("close request did not stop the handshake")
	}
}

func TestSecurityCloseSkipsRemovedState(t *testing.T) {
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("close-removed-test")}))
	conn := &securityCloseConn{securityConn: &securityConn{id: 1}}
	wrapped := g.security.wrap(conn).(*securedConn)
	g.security.remove(1)
	g.security.closeSecureConn(1, wrapped)
	if conn.calls.Load() != 0 {
		t.Fatal("removed session was closed by stale work")
	}
}

func TestSecurityCloseSkipsReplacedState(t *testing.T) {
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("close-replaced-test")}))
	old := &securityCloseConn{securityConn: &securityConn{id: 1}}
	wrapped := g.security.wrap(old).(*securedConn)
	g.security.remove(1)
	current := &securityCloseConn{securityConn: &securityConn{id: 1}}
	g.security.wrap(current)
	t.Cleanup(func() { g.security.remove(1) })
	g.security.closeSecureConn(1, wrapped)
	if old.calls.Load() != 0 || current.calls.Load() != 0 {
		t.Fatal("replacement session was affected by stale work")
	}
}

func TestSecurityShutdownRejectsLateClose(t *testing.T) {
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("shutdown-late-test")}))
	conn := &securityCloseConn{securityConn: &securityConn{id: 1}}
	g.security.wrap(conn)
	g.security.shutdown()
	g.security.closeConn(1)
	g.security.closers.Wait()
	if conn.calls.Load() != 0 {
		t.Fatal("shutdown must reject a security close started after it")
	}
}

type panickingSecurityConn struct{ *securityConn }

func (*panickingSecurityConn) Close(...bool) error {
	panic("test transport close panic")
}

func TestSecurityClosePanicCompletesWaitGroup(t *testing.T) {
	g := NewGate(WithSessionSecurity(&securitySigner{key: []byte("close-panic-test")}))
	conn := &panickingSecurityConn{securityConn: &securityConn{id: 1}}
	g.security.wrap(conn)
	t.Cleanup(func() { g.security.remove(1) })
	g.security.closeConn(1)
	joined := make(chan struct{})
	go func() {
		g.security.closers.Wait()
		close(joined)
	}()
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("close panic did not complete the wait group")
	}
}

func TestSecurityExpiryKeepsConfirmedConnection(t *testing.T) {
	g, conn, _, _ := newSecurityTestPeer(t, 10)
	t.Cleanup(func() { g.security.remove(10) })
	g.security.expire(10)
	g.security.closers.Wait()
	if conn.closed.Load() {
		t.Fatal("handshake expiry closed a confirmed connection")
	}
}
