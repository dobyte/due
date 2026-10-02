package drpc

import (
	"sync"

	"github.com/dobyte/due/v2/core/buffer"
)

type pending struct {
	calls []*calls // Shards
}

func newPending() *pending {
	p := &pending{calls: make([]*calls, 64)}

	for i := 0; i < len(p.calls); i++ {
		p.calls[i] = &calls{calls: make(map[uint64]chan *buffer.Bytes)}
	}

	return p
}

// reply delivers buf to the pending call identified by seq.
func (p *pending) reply(seq uint64, buf *buffer.Bytes) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].reply(seq, buf)
}

// store registers ch as the pending call identified by seq.
func (p *pending) store(seq uint64, ch chan *buffer.Bytes) {
	p.calls[int(seq%uint64(len(p.calls)))].store(seq, ch)
}

// delete removes and closes the pending call identified by seq.
func (p *pending) delete(seq uint64) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].delete(seq)
}

// closeAll closes all pending calls, waking every waiter and releasing resources.
func (p *pending) closeAll() {
	for _, c := range p.calls {
		c.closeAll()
	}
}

type calls struct {
	mu    sync.Mutex                    // Lock
	calls map[uint64]chan *buffer.Bytes // Synchronization channels
	_     [48]byte                      // Cache-line padding up to 64 bytes, eliminating false sharing between adjacent shards
}

// reply delivers buf to the call identified by seq, reporting whether it was pending.
func (p *calls) reply(seq uint64, buf *buffer.Bytes) (ok bool) {
	var ch chan *buffer.Bytes

	p.mu.Lock()
	if ch, ok = p.calls[seq]; ok {
		delete(p.calls, seq)
		ch <- buf
	}
	p.mu.Unlock()

	return
}

// store registers ch as the call identified by seq.
func (p *calls) store(seq uint64, ch chan *buffer.Bytes) {
	p.mu.Lock()
	p.calls[seq] = ch
	p.mu.Unlock()
}

// delete removes and closes the call identified by seq.
func (p *calls) delete(seq uint64) (ok bool) {
	var ch chan *buffer.Bytes

	p.mu.Lock()
	if ch, ok = p.calls[seq]; ok {
		close(ch)
		delete(p.calls, seq)
	}
	p.mu.Unlock()

	return
}

// closeAll closes all pending calls.
func (p *calls) closeAll() {
	p.mu.Lock()
	for seq, ch := range p.calls {
		close(ch)
		delete(p.calls, seq)
	}
	p.mu.Unlock()
}
