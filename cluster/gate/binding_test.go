package gate

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
)

// blockingBindLocator pauses one user's location write after the local binding succeeds.
type blockingBindLocator struct {
	*gateLocator
	uid     int64
	entered chan struct{}
	resume  chan struct{}
}

// failingFirstBindLocator models an old remote request failing after a replacement local bind.
type failingFirstBindLocator struct {
	*gateLocator
	entered chan struct{}
	resume  chan struct{}
	calls   atomic.Int32
}

func (l *failingFirstBindLocator) BindGate(ctx context.Context, uid int64, gid string) error {
	if l.calls.Add(1) == 1 {
		close(l.entered)
		select {
		case <-l.resume:
			return errors.ErrInvalidArgument
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return l.gateLocator.BindGate(ctx, uid, gid)
}

func TestGateFailedBindPreservesReplacement(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l := &failingFirstBindLocator{gateLocator: f.locator,
		entered: make(chan struct{}), resume: make(chan struct{})}
	g.opts.locator = l
	first, second := &securityConn{id: 1}, &securityConn{id: 2}
	g.handleConnect(first)
	g.handleConnect(second)
	p := &provider{gate: g}
	a, b := make(chan error, 1), make(chan error, 1)
	go func() { a <- p.Bind(ctx, 1, 100) }()
	select {
	case <-l.entered:
	case <-ctx.Done():
		t.Fatal("first request did not reach the locator")
	}
	go func() { b <- p.Bind(ctx, 2, 100) }()
	for second.UID() != 100 {
		select {
		case <-ctx.Done():
			t.Fatal("replacement did not reach the local binding")
		case <-time.After(time.Millisecond):
		}
	}
	close(l.resume)
	if err := <-a; !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("first request returned %v", err)
	}
	if err := <-b; err != nil {
		t.Fatal(err)
	}
	if !g.session.isBound(2, 100) {
		t.Fatal("the old request rolled back the replacement")
	}
	g.handleDisconnect(first)
	if gid, _ := l.LocateGate(ctx, 100); gid != g.opts.id {
		t.Fatalf("the old disconnect cleared the replacement's location: %q", gid)
	}
	g.handleDisconnect(second)
}

func TestGateChangeUIDCleansPreviousLocation(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	conn := &securityConn{id: 1}
	g.handleConnect(conn)
	p := &provider{gate: g}
	ctx := context.Background()
	if err := p.Bind(ctx, 1, 100); err != nil {
		t.Fatal(err)
	}
	if err := p.Bind(ctx, 1, 200); err != nil {
		t.Fatal(err)
	}
	if gid, _ := f.locator.LocateGate(ctx, 100); gid != "" {
		t.Fatalf("previous UID still located on gate %q", gid)
	}
	if !g.session.isBound(1, 200) {
		t.Fatal("new UID is not bound")
	}
	g.handleDisconnect(conn)
	waitForCondition(t, 3*time.Second, func() bool {
		gid, _ := f.locator.LocateGate(ctx, 200)
		return gid == ""
	}, "new UID is still located after disconnect")
}

func TestGateChangeUIDStopsOnCleanupFailure(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	conn := &securityConn{id: 1}
	g.handleConnect(conn)
	p := &provider{gate: g}
	ctx := context.Background()
	if err := p.Bind(ctx, 1, 100); err != nil {
		t.Fatal(err)
	}
	f.locator.unbindGateErr = errors.ErrInvalidArgument
	if err := p.Bind(ctx, 1, 200); !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("cleanup failure returned %v", err)
	}
	if !g.session.isBound(1, 100) || g.session.isBound(1, 200) {
		t.Fatal("failed cleanup changed the user's local identity")
	}
	f.locator.unbindGateErr = nil
	g.handleDisconnect(conn)
}

func (l *blockingBindLocator) BindGate(ctx context.Context, uid int64, gid string) error {
	if uid == l.uid {
		close(l.entered)
		select {
		case <-l.resume:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return l.gateLocator.BindGate(ctx, uid, gid)
}

// blockingUnbindLocator pauses the location cleanup of one user.
type blockingUnbindLocator struct {
	*gateLocator
	uid     int64
	entered chan struct{}
	resume  chan struct{}
}

func (l *blockingUnbindLocator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	if uid == l.uid {
		close(l.entered)
		select {
		case <-l.resume:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return l.gateLocator.UnbindGate(ctx, uid, gid)
}

// TestGateDisconnectDoesNotWaitForLocationCleanup checks that the close callback returns while the
// location cleanup is still in flight.
func TestGateDisconnectDoesNotWaitForLocationCleanup(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l := &blockingUnbindLocator{gateLocator: f.locator, uid: 100,
		entered: make(chan struct{}), resume: make(chan struct{})}
	g.opts.locator = l
	conn := &securityConn{id: 1}
	g.handleConnect(conn)
	if err := (&provider{gate: g}).Bind(ctx, 1, 100); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		g.handleDisconnect(conn)
		close(done)
	}()
	select {
	case <-l.entered:
	case <-ctx.Done():
		t.Fatal("cleanup did not reach the locator")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnect waited for the location cleanup")
	}

	close(l.resume)

	waitForCondition(t, 3*time.Second, func() bool {
		gid, _ := l.LocateGate(ctx, 100)
		return gid == ""
	}, "disconnected user is still located on the gate")
}

func TestGateBindConcurrentDisconnect(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l := &blockingBindLocator{gateLocator: f.locator, uid: 100,
		entered: make(chan struct{}), resume: make(chan struct{})}
	g.opts.locator = l
	conn := &securityConn{id: 1}
	g.handleConnect(conn)
	bound := make(chan error, 1)
	go func() { bound <- (&provider{gate: g}).Bind(ctx, 1, 100) }()
	select {
	case <-l.entered:
	case <-ctx.Done():
		t.Fatal("binding did not reach the locator")
	}
	disconnected := make(chan struct{})
	go func() {
		g.handleDisconnect(conn)
		close(disconnected)
	}()
	// Wait until the local removal has happened while the location write is still blocked.
	for {
		if online, _ := g.session.has(cluster.Conn, 1); !online {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("disconnect did not remove the local connection")
		case <-time.After(time.Millisecond):
		}
	}
	close(l.resume)
	select {
	case err := <-bound:
		if err != nil && !errors.Is(err, errors.ErrNotFoundSession) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("binding did not finish")
	}
	select {
	case <-disconnected:
	case <-ctx.Done():
		t.Fatal("disconnect did not finish")
	}
	waitForCondition(t, 3*time.Second, func() bool {
		gid, _ := l.LocateGate(ctx, 100)
		return gid == ""
	}, "disconnected user is still located on the gate")
}

// disconnectOnCloseConn models the synchronous disconnect callback made by network.Conn.Close.
type disconnectOnCloseConn struct {
	*securityConn
	onClose func()
}

func (c *disconnectOnCloseConn) Close(...bool) error {
	c.onClose()
	return nil
}

func TestGateBindReplacementDoesNotDeadlock(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	old := &disconnectOnCloseConn{securityConn: &securityConn{id: 1}}
	old.onClose = func() { g.handleDisconnect(old) }
	g.handleConnect(old)
	if err := g.session.bind(1, 100); err != nil {
		t.Fatal(err)
	}
	conn := &securityConn{id: 2}
	g.handleConnect(conn)
	done := make(chan error, 1)
	go func() { done <- (&provider{gate: g}).Bind(context.Background(), 2, 100) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("binding waited on the old connection's disconnect while holding the user lock")
	}
	if gid, _ := f.locator.LocateGate(context.Background(), 100); gid != g.opts.id {
		t.Fatalf("replacement user not located on gate: %q", gid)
	}
	g.handleDisconnect(conn)
}

func TestGateBindDisconnectedBeforeLocationWrite(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	old := &disconnectOnCloseConn{securityConn: &securityConn{id: 1}}
	g.handleConnect(old)
	if err := g.session.bind(1, 100); err != nil {
		t.Fatal(err)
	}
	conn := &securityConn{id: 2}
	g.handleConnect(conn)
	// Both disconnects finish after the local bind, before Session.Bind returns to the provider.
	old.onClose = func() {
		g.handleDisconnect(old)
		g.handleDisconnect(conn)
	}
	if err := (&provider{gate: g}).Bind(context.Background(), 2, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("binding a disconnected connection returned %v", err)
	}
	waitForCondition(t, 3*time.Second, func() bool {
		gid, _ := f.locator.LocateGate(context.Background(), 100)
		return gid == ""
	}, "disconnected user is still located on the gate")
}

func TestGateBindDifferentUsersProceedIndependently(t *testing.T) {
	f := newGateFixture(t)
	g := f.gate
	g.state.Store(int32(cluster.Work))
	t.Cleanup(g.cancel)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l := &blockingBindLocator{gateLocator: f.locator, uid: 100,
		entered: make(chan struct{}), resume: make(chan struct{})}
	g.opts.locator = l
	first := &securityConn{id: 1}
	second := &securityConn{id: 2}
	g.handleConnect(first)
	g.handleConnect(second)
	p := &provider{gate: g}
	done := make(chan error, 1)
	go func() { done <- p.Bind(ctx, 1, 100) }()
	select {
	case <-l.entered:
	case <-ctx.Done():
		t.Fatal("first user did not reach the locator")
	}
	other := make(chan error, 1)
	go func() { other <- p.Bind(ctx, 2, 200) }()
	select {
	case err := <-other:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("unrelated user's binding blocked behind the first user")
	}
	close(l.resume)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	g.handleDisconnect(first)
	g.handleDisconnect(second)
}

func TestBindingLockCacheLineAlignment(t *testing.T) {
	const cacheLineSize = 64

	if size := unsafe.Sizeof(bindingLock{}); size%cacheLineSize != 0 {
		t.Fatalf("bindingLock size %d is not a multiple of the cache line size", size)
	}

	var gate Gate

	if off := unsafe.Offsetof(gate.userBindings); off%cacheLineSize != 0 {
		t.Fatalf("userBindings offset %d is not cache line aligned", off)
	}
	if off := unsafe.Offsetof(gate.connBindings); off%cacheLineSize != 0 {
		t.Fatalf("connBindings offset %d is not cache line aligned", off)
	}
}
