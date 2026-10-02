package link

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/session"
)

// collector accumulates the payloads a gate provider receives.
type collector struct {
	mu       sync.Mutex
	payloads [][]byte
}

// record stores a copy of the payload carried by buf.
func (c *collector) record(buf buffer.Buffer) {
	c.mu.Lock()
	c.payloads = append(c.payloads, append([]byte(nil), buf.Bytes()...))
	c.mu.Unlock()
}

// snapshot returns the number of recorded payloads and the payloads themselves.
func (c *collector) snapshot() (int, [][]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.payloads), append([][]byte(nil), c.payloads...)
}

// waitPayloads waits until the collector holds want payloads and reports them.
func (c *collector) waitPayloads(t *testing.T, want int) [][]byte {
	t.Helper()

	if !waitFor(t, 3*time.Second, func() bool {
		n, _ := c.snapshot()
		return n == want
	}) {
		n, _ := c.snapshot()
		t.Fatalf("collected payloads = %d, want %d", n, want)
	}

	_, payloads := c.snapshot()

	return payloads
}

// assertIdenticalPayloads fails when the payloads are empty or are not byte identical, which would
// mean that concurrent encoding of the shared buffer produced inconsistent results.
func assertIdenticalPayloads(t *testing.T, payloads [][]byte) {
	t.Helper()

	for i, payload := range payloads {
		if len(payload) == 0 {
			t.Fatalf("payload %d is empty", i)
		}
		if !bytes.Equal(payload, payloads[0]) {
			t.Errorf("payload %d differs from payload 0: %q vs %q", i, payload, payloads[0])
		}
	}
}

// TestGateLinkerBroadcastFanOut verifies a live broadcast across several gates, where every gate
// encodes the one shared packed buffer concurrently.
func TestGateLinkerBroadcastFanOut(t *testing.T) {
	tests := []struct {
		name      string
		ack       bool
		wantTotal int64
	}{
		{name: "ack", ack: true, wantTotal: 6}, // two gates, the stub reports three sessions each
		{name: "no ack", ack: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collected := &collector{}
			capture := func(_ context.Context, _ session.Kind, _ bool, buf buffer.Buffer) (int64, error) {
				collected.record(buf)
				return 3, nil
			}

			first := &gateProviderStub{broadcastFn: capture}
			second := &gateProviderStub{broadcastFn: capture}
			_, addr1 := startGateServer(t, first)
			_, addr2 := startGateServer(t, second)

			linker := NewGateLinker(context.Background(), baseLinkOptions())
			linker.dispatcher.ReplaceServices(gateService("gate-1", addr1), gateService("gate-2", addr2))

			total, err := linker.Broadcast(context.Background(), &BroadcastArgs{
				Kind:    session.Conn,
				Message: &Message{Route: 1, Data: []byte("hello")},
				Ack:     tt.ack,
			})
			if err != nil {
				t.Fatalf("Broadcast failed: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}

			if !waitFor(t, 3*time.Second, func() bool {
				return first.broadcastCount.Load() == 1 && second.broadcastCount.Load() == 1
			}) {
				t.Fatalf("broadcast counts = (%d, %d), want (1, 1)", first.broadcastCount.Load(), second.broadcastCount.Load())
			}

			assertIdenticalPayloads(t, collected.waitPayloads(t, 2))
		})
	}
}

// TestGateLinkerPublishFanOut verifies a live publish across several gates, where every gate encodes
// the one shared packed buffer concurrently.
func TestGateLinkerPublishFanOut(t *testing.T) {
	tests := []struct {
		name      string
		ack       bool
		wantTotal int64
	}{
		{name: "ack", ack: true, wantTotal: 4}, // two gates, the stub reports two sessions each
		{name: "no ack", ack: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collected := &collector{}
			capture := func(_ context.Context, _ string, _ bool, buf buffer.Buffer) (int64, error) {
				collected.record(buf)
				return 2, nil
			}

			first := &gateProviderStub{publishFn: capture}
			second := &gateProviderStub{publishFn: capture}
			_, addr1 := startGateServer(t, first)
			_, addr2 := startGateServer(t, second)

			linker := NewGateLinker(context.Background(), baseLinkOptions())
			linker.dispatcher.ReplaceServices(gateService("gate-1", addr1), gateService("gate-2", addr2))

			total, err := linker.Publish(context.Background(), &PublishArgs{
				Channel: "room",
				Message: &Message{Route: 1, Data: []byte("hello")},
				Ack:     tt.ack,
			})
			if err != nil {
				t.Fatalf("Publish failed: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}

			if !waitFor(t, 3*time.Second, func() bool {
				return first.publishCount.Load() == 1 && second.publishCount.Load() == 1
			}) {
				t.Fatalf("publish counts = (%d, %d), want (1, 1)", first.publishCount.Load(), second.publishCount.Load())
			}

			assertIdenticalPayloads(t, collected.waitPayloads(t, 2))
		})
	}
}

// TestGateLinkerMulticastIndirectFanOut verifies an indirect multicast whose targets are located on
// different gates, so every target is pushed through its own client concurrently.
func TestGateLinkerMulticastIndirectFanOut(t *testing.T) {
	tests := []struct {
		name      string
		ack       bool
		wantTotal int64
	}{
		{name: "ack", ack: true, wantTotal: 2},
		{name: "no ack", ack: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := &gateProviderStub{}
			second := &gateProviderStub{}
			_, addr1 := startGateServer(t, first)
			_, addr2 := startGateServer(t, second)

			linker := newGateLinker()
			linker.dispatcher.ReplaceServices(gateService("gate-1", addr1), gateService("gate-2", addr2))
			linker.sources.Store(int64(200), "gate-1")
			linker.sources.Store(int64(201), "gate-2")

			total, err := linker.Multicast(context.Background(), &MulticastArgs{
				Kind:    session.User,
				Targets: []int64{200, 201},
				Message: &Message{Route: 1, Data: []byte("hello")},
				Ack:     tt.ack,
			})
			if err != nil {
				t.Fatalf("Multicast failed: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}

			if !waitFor(t, 3*time.Second, func() bool {
				return first.pushCount.Load() == 1 && second.pushCount.Load() == 1
			}) {
				t.Fatalf("push counts = (%d, %d), want (1, 1)", first.pushCount.Load(), second.pushCount.Load())
			}
		})
	}
}
