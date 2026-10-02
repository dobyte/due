package kcp

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/xtaci/kcp-go/v5"
)

type serverConnMgr struct {
	cid        atomic.Int64 // Connection ID
	total      atomic.Int64 // Total number of connections
	server     *server      // Server
	connPool   sync.Pool    // Connection pool
	partitions []*partition // Connection partitions
}

// newServerConnMgr returns a new connection manager.
//
// It initializes the connection pool and a set of partitions sized by the number of CPUs.
func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.partitions = make([]*partition, runtime.NumCPU()*2)
	cm.connPool = sync.Pool{New: func() any {
		return &serverConn{
			attr:       &attr{},
			connMgr:    cm,
			dueBuffers: make([]buffer.Buffer, 0, maxBatchWriteNum),
			netBuffers: make(net.Buffers, 0, 2*maxBatchWriteNum),
		}
	}}

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[*kcp.UDPSession]*serverConn)}
	}

	return cm
}

// close closes every connection.
//
// It walks all partitions in parallel and closes the connections in each of them, waiting for all
// of them to finish.
func (cm *serverConnMgr) close() {
	wg, _ := taskpool.WithContext(context.Background())

	for _, p := range cm.partitions {
		wg.Go(p.close)
	}

	if err := wg.Wait(); err != nil {
		log.Warnf("close connections error: %v", err)
	}
}

// open opens the manager for incoming connections.
//
// It clears the stop flag of every partition and is called on each server start so that the server
// can be restarted.
func (cm *serverConnMgr) open() {
	for _, p := range cm.partitions {
		p.rw.Lock()
		p.stopped = false
		p.rw.Unlock()
	}
}

// allocateConn allocates a connection for c.
//
// It increments the total connection count and checks it against the limit, rolling the count back
// when the limit is exceeded. It then takes a connection object from the pool, initializes it and
// stores it in a partition.
func (cm *serverConnMgr) allocateConn(c *kcp.UDPSession) error {
	if cm.total.Add(1) > int64(cm.server.opts.maxConnNum) {
		cm.total.Add(-1)
		return errors.ErrTooManyConnection
	}

	conn := cm.connPool.Get().(*serverConn)

	if !conn.init(c) {
		// A partition refuses storage while the server is shutting down; roll the count back,
		// return the connection object and let the caller close the underlying connection.
		cm.total.Add(-1)
		cm.connPool.Put(conn)
		return errors.ErrServerClosed
	}

	return nil
}

// storeConn stores a connection.
//
// It hashes the connection pointer to pick a partition, which refuses storage when it has stopped.
func (cm *serverConnMgr) storeConn(c *kcp.UDPSession, conn *serverConn) bool {
	return cm.partitions[cm.connHash(c)].store(c, conn)
}

// recycleConn recycles a connection.
//
// It removes the connection from its partition, resets it, returns it to the pool and decrements
// the total connection count.
func (cm *serverConnMgr) recycleConn(c *kcp.UDPSession) {
	if conn, ok := cm.partitions[cm.connHash(c)].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// connHash computes the partition index for a connection from its pointer.
//
// It bit-mixes the pointer address before taking the modulus so that low-order address alignment
// does not make the partition distribution uneven.
func (cm *serverConnMgr) connHash(c *kcp.UDPSession) int {
	return int(cm.mixPointer(uintptr(unsafe.Pointer(c))) % uintptr(len(cm.partitions)))
}

// mixPointer scrambles a pointer address so that low-order address alignment cannot make the
// modulus produce an uneven partition distribution.
func (cm *serverConnMgr) mixPointer(p uintptr) uintptr {
	x := uint64(p)
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33

	return uintptr(x)
}

type partition struct {
	rw          sync.RWMutex
	connections map[*kcp.UDPSession]*serverConn
	stopped     bool     // Whether new connections are refused; set when the partition closes and cleared on server restart
	_           [31]byte // Padding to a 64-byte cache line to avoid false sharing between neighboring partitions
}

// store stores a connection.
//
// It writes the connection mapping into the partition. A stopped partition refuses the write so
// that in-flight connections are not leaked while the server is shutting down.
func (p *partition) store(c *kcp.UDPSession, conn *serverConn) bool {
	p.rw.Lock()

	if p.stopped {
		p.rw.Unlock()
		return false
	}

	p.connections[c] = conn
	p.rw.Unlock()

	return true
}

// delete removes a connection and returns it together with whether it existed.
func (p *partition) delete(c *kcp.UDPSession) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[c]
	if ok {
		delete(p.connections, c)
	}
	p.rw.Unlock()

	return conn, ok
}

// close closes every connection in the partition.
//
// It first sets the stop flag to block new connections, then closes the partition's connections
// serially while the outer level drives the partitions in parallel; this avoids submitting a huge
// number of blocking tasks to the task pool at once. A connection closed concurrently by another
// path is a normal race and is not treated as an error. It returns the first error encountered
// when closing a connection.
func (p *partition) close() error {
	p.rw.Lock()
	p.stopped = true
	conns := make([]*serverConn, 0, len(p.connections))
	for _, conn := range p.connections {
		conns = append(conns, conn)
	}
	p.rw.Unlock()

	var firstErr error

	for _, conn := range conns {
		err := conn.Close()
		if err == nil || errors.Is(err, errors.ErrConnectionNotOpened) || errors.Is(err, errors.ErrConnectionClosed) {
			continue
		}

		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
