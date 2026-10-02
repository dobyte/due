package tcp

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/pires/go-proxyproto"
)

type serverConnMgr struct {
	cid        atomic.Int64 // Connection ID
	total      atomic.Int64 // Total number of connections
	server     *server      // Server
	connPool   sync.Pool    // Connection pool
	partitions []*partition // Connection management
}

// newServerConnMgr returns a new connection manager.
//
// It initializes the connection pool, the task pool and the partition structure that grows
// dynamically with the number of CPUs.
func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any {
		return &serverConn{
			attr:       &attr{},
			connMgr:    cm,
			reader:     bufio.NewReaderSize(nil, server.opts.readBufferSize),
			dueBuffers: make([]buffer.Buffer, 0, maxBatchWriteNum),
			netBuffers: make(net.Buffers, 0, 2*maxBatchWriteNum),
		}
	}}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[net.Conn]*serverConn)}
	}

	return cm
}

// close closes all connections.
//
// It traverses every partition in parallel, closes the connections in each partition one by one and
// waits for completion.
func (cm *serverConnMgr) close() {
	wg, _ := taskpool.WithContext(context.Background())

	for _, p := range cm.partitions {
		wg.Go(p.close)
	}

	if err := wg.Wait(); err != nil {
		log.Warnf("close connections error: %v", err)
	}
}

// open allows connections to be accepted.
//
// It clears the stopped flag of every partition and is called on each server start to support a
// restart.
func (cm *serverConnMgr) open() {
	for _, p := range cm.partitions {
		p.rw.Lock()
		p.stopped = false
		p.rw.Unlock()
	}
}

// allocateConn allocates a connection.
//
// It increments the total connection count and checks it against the limit, rolling the count back
// when the limit is exceeded; it takes a connection object from the connection pool, initializes it
// and stores it in a partition.
func (cm *serverConnMgr) allocateConn(c net.Conn) error {
	if cm.total.Add(1) > int64(cm.server.opts.maxConnNum) {
		cm.total.Add(-1)
		return errors.ErrTooManyConnection
	}

	conn := cm.connPool.Get().(*serverConn)

	if !conn.init(c) {
		// The partition refused the store while the server was closing, so roll the count back and return the connection object; the caller closes the underlying connection.
		cm.total.Add(-1)
		cm.connPool.Put(conn)
		return errors.ErrServerClosed
	}

	return nil
}

// storeConn stores the connection.
//
// It stores the connection in the partition selected by hashing the connection pointer, and refuses
// the store when the partition has stopped.
func (cm *serverConnMgr) storeConn(c net.Conn, conn *serverConn) bool {
	return cm.partitions[cm.connHash(c)].store(c, conn)
}

// recycleConn recycles the connection.
//
// It removes the connection object from the partition, resets it, returns it to the connection pool
// and decrements the total connection count.
func (cm *serverConnMgr) recycleConn(c net.Conn) {
	if conn, ok := cm.partitions[cm.connHash(c)].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// connHash computes the hash from the connection pointer.
//
// It mixes the pointer address of the connection object and takes the modulus to determine the
// partition index, avoiding an uneven partition distribution caused by object address alignment.
func (cm *serverConnMgr) connHash(c net.Conn) int {
	var p uintptr

	switch cc := c.(type) {
	case *proxyproto.Conn:
		p = uintptr(unsafe.Pointer(cc))
	case *tls.Conn:
		p = uintptr(unsafe.Pointer(cc))
	case *net.TCPConn:
		p = uintptr(unsafe.Pointer(cc))
	default:
		if v := reflect.ValueOf(c); v.Kind() == reflect.Ptr {
			p = v.Pointer()
		}
		// A non-pointer connection implementation has no stable address, so keep the zero value and assign it to the first partition.
	}

	return int(cm.mixPointer(p) % uintptr(len(cm.partitions)))
}

// mixPointer spreads the pointer address to avoid an uneven partition distribution after the
// modulus caused by the low-bit alignment of object addresses.
func (cm *serverConnMgr) mixPointer(p uintptr) uintptr {
	x := uint64(p)
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33

	return uintptr(x)
}

type partition struct {
	rw          sync.RWMutex
	connections map[net.Conn]*serverConn
	stopped     bool     // Whether new connections are no longer accepted; set when the partition closes and reset when the server restarts
	_           [31]byte // Padding to a 64-byte cache line to avoid false sharing between adjacent partitions
}

// store stores the connection.
//
// It writes the connection mapping into the partition and refuses the write when the partition has
// stopped, avoiding a leak of in-flight connections while the server is closing.
func (p *partition) store(c net.Conn, conn *serverConn) bool {
	p.rw.Lock()

	if p.stopped {
		p.rw.Unlock()
		return false
	}

	p.connections[c] = conn
	p.rw.Unlock()

	return true
}

// delete deletes the connection.
//
// It removes the connection object from the partition and returns it.
func (p *partition) delete(c net.Conn) (*serverConn, bool) {
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
// It first sets the stopped flag to block new connection writes and then closes every connection in
// the partition serially; the outer layer drives the partitions in parallel, which avoids
// submitting a huge number of blocking tasks to the task pool at once. Closing the same connection
// concurrently from another path is a normal race and is not treated as an error.
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
