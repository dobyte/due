package drpc

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// TestPendingStoreAndReply verifies that a stored call receives its response and is then removed.
func TestPendingStoreAndReply(t *testing.T) {
	p := newPending()
	ch := make(chan *buffer.Bytes, 1)

	p.store(3, ch)

	if ok := p.reply(3, buffer.NewBytes([]byte("payload"))); !ok {
		t.Fatal("expect reply to find the pending call")
	}

	if got := <-ch; string(got.Bytes()) != "payload" {
		t.Fatalf("unexpected response: %s", got.Bytes())
	}

	// The call is removed once it has been replied to.
	if ok := p.reply(3, buffer.NewBytes(nil)); ok {
		t.Fatal("expect reply to miss an already replied call")
	}
}

// TestPendingDelete verifies that delete removes and closes a pending call.
func TestPendingDelete(t *testing.T) {
	p := newPending()
	ch := make(chan *buffer.Bytes, 1)
	p.store(4, ch)

	if ok := p.delete(4); !ok {
		t.Fatal("expect delete to find the pending call")
	}
	if _, ok := <-ch; ok {
		t.Fatal("expect the call channel to be closed")
	}
	if ok := p.delete(4); ok {
		t.Fatal("expect delete to miss a removed call")
	}
}

// TestPendingCloseAll verifies that closeAll closes the calls of every shard.
func TestPendingCloseAll(t *testing.T) {
	p := newPending()
	chans := make([]chan *buffer.Bytes, 0, len(p.calls))

	for i := range p.calls {
		ch := make(chan *buffer.Bytes, 1)
		p.calls[i].store(uint64(i), ch)
		chans = append(chans, ch)
	}

	p.closeAll()

	for i, ch := range chans {
		if _, ok := <-ch; ok {
			t.Fatalf("expect shard %d call to be closed", i)
		}
	}
}

// TestCallsReplyMissing verifies that reply reports false and keeps the buffer untouched when the
// sequence is not pending.
func TestCallsReplyMissing(t *testing.T) {
	c := &calls{calls: make(map[uint64]chan *buffer.Bytes)}

	if ok := c.reply(9, buffer.NewBytes(nil)); ok {
		t.Fatal("expect reply to miss an unknown sequence")
	}
}
