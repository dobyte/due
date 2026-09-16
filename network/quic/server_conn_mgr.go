package quic

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/quic-go/quic-go"
)

type managedConn struct {
	qc   *quic.Conn
	conn *conn
}

type partition struct {
	mu          sync.Mutex
	connections map[int64]managedConn
}

// serverConnMgr accounts for both pending streams and active connections.
type serverConnMgr struct {
	server     *server
	total      atomic.Int64
	partitions []partition
	closed     atomic.Bool
	closeOnce  sync.Once
}

func newServerConnMgr(s *server) *serverConnMgr {
	m := &serverConnMgr{server: s, partitions: make([]partition, max(1, runtime.GOMAXPROCS(0)*2))}
	for i := range m.partitions {
		m.partitions[i].connections = make(map[int64]managedConn)
	}
	return m
}

func (m *serverConnMgr) reserve(qc *quic.Conn) (int64, bool) {
	for {
		total := m.total.Load()
		if m.closed.Load() || total >= int64(m.server.opts.maxConnNum) {
			return 0, false
		}
		if m.total.CompareAndSwap(total, total+1) {
			break
		}
	}
	id := m.server.id.Add(1)
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	defer p.mu.Unlock()
	if m.closed.Load() {
		m.total.Add(-1)
		return 0, false
	}
	p.connections[id] = managedConn{qc: qc}
	return id, true
}

func (m *serverConnMgr) attach(id int64, c *conn) bool {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.connections[id]
	if !ok || m.closed.Load() {
		return false
	}
	entry.conn = c
	p.connections[id] = entry
	return true
}

func (m *serverConnMgr) remove(id int64) {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	if _, ok := p.connections[id]; ok {
		delete(p.connections, id)
		m.total.Add(-1)
	}
	p.mu.Unlock()
}

func (m *serverConnMgr) close() {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		var wg sync.WaitGroup
		for i := range m.partitions {
			p := &m.partitions[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.mu.Lock()
				entries := make([]managedConn, 0, len(p.connections))
				for id, entry := range p.connections {
					entries = append(entries, entry)
					delete(p.connections, id)
					m.total.Add(-1)
				}
				p.mu.Unlock()
				for _, entry := range entries {
					if entry.conn != nil {
						_ = entry.conn.forceClose()
					} else {
						_ = entry.qc.CloseWithError(0, "server stopped")
					}
				}
			}()
		}
		wg.Wait()
	})
}
