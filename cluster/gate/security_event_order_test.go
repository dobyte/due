package gate

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/packet"
)

// newEventOrderGate builds a gate with session security enabled. The connection events are delivered
// through the trigger hook, which the tests install to observe the delivery order.
func newEventOrderGate(t *testing.T) (*Gate, *securitySigner) {
	t.Helper()

	f := newGateFixture(t)
	g := f.gate
	t.Cleanup(g.cancel)

	signer := &securitySigner{key: []byte("gate-event-order-test")}
	g.security = newSecurityManager(g, &securityOptions{signer: signer, suites: []aead.Suite{aesgcm.Suite}}, 0)
	g.state.Store(int32(cluster.Work))

	return g, signer
}

// TestGateDisconnectEventWaitsForConnectDelivery verifies that a Disconnect event cannot overtake a
// Connect delivery that is still in flight for the same connection.
func TestGateDisconnectEventWaitsForConnectDelivery(t *testing.T) {
	var (
		events  = make(chan cluster.Event, 4)
		started = make(chan struct{})
		release = make(chan struct{})
	)

	g, signer := newEventOrderGate(t)

	// The Connect delivery blocks until the test releases it, which keeps the delivery in flight while
	// the connection is being removed.
	g.security.trigger = func(_ context.Context, event cluster.Event, _, _ int64) {
		if event == cluster.Connect {
			close(started)
			<-release
		}

		events <- event
	}

	conn := &securityConn{id: 1, pushed: make(chan []byte, 4)}
	g.handleConnect(conn)

	client := handshake.NewClient(signer, aesgcm.Suite)

	hello, err := client.Hello()
	if err != nil {
		t.Fatalf("client hello failed: %v", err)
	}

	injectHandshake(t, g, conn, hello)

	_, finished, err := client.Handle(nextHandshake(t, conn.next(t)))
	if err != nil {
		t.Fatalf("client handle failed: %v", err)
	}

	finishedBuf, err := packet.PackHandshake(finished)
	if err != nil {
		t.Fatalf("pack handshake frame failed: %v", err)
	}

	confirmed := make(chan struct{})
	go func() {
		g.security.handleHandshake(conn, finishedBuf)
		close(confirmed)
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("the Connect delivery did not start")
	}

	disconnected := make(chan struct{})
	go func() {
		g.handleDisconnect(conn)
		close(disconnected)
	}()

	select {
	case event := <-events:
		t.Fatalf("event %v was delivered before the in-flight Connect delivery finished", event)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-confirmed:
	case <-time.After(3 * time.Second):
		t.Fatal("the Connect delivery did not finish")
	}

	for _, expected := range []cluster.Event{cluster.Connect, cluster.Disconnect} {
		select {
		case event := <-events:
			if event != expected {
				t.Fatalf("event order = %v, want %v", event, expected)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("event %v was not delivered", expected)
		}
	}

	select {
	case <-disconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("the disconnect callback did not finish")
	}
}

// TestGateDisconnectEventSkippedWithoutConnect verifies that a connection removed before its Connect
// event was committed produces no Disconnect event.
func TestGateDisconnectEventSkippedWithoutConnect(t *testing.T) {
	events := make(chan cluster.Event, 4)
	g, _ := newEventOrderGate(t)
	g.security.trigger = func(_ context.Context, event cluster.Event, _, _ int64) {
		events <- event
	}

	conn := &securityConn{id: 1, pushed: make(chan []byte, 4)}
	g.handleConnect(conn)
	g.handleDisconnect(conn)

	select {
	case event := <-events:
		t.Fatalf("unexpected event %v for a connection without a Connect event", event)
	case <-time.After(100 * time.Millisecond):
	}
}
