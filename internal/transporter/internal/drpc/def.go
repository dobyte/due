package drpc

import (
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
)

const (
	connClosed int32 = iota // Connection closed
	connOpened              // Connection opened
	connAlived              // Connection alive
	connHanged              // Connection hung
)

const (
	heartbeatInterval = 10 * time.Second // Heartbeat interval
	maxBatchWriteNum  = 64               // Maximum number of messages written in one batch
	maxRetentionTime  = 1 * time.Second  // Maximum retention time
)

// closedQueue is a cached closed queue.
type closedQueue struct {
	time  time.Time                   // Close time
	queue *queue.Queue[buffer.Buffer] // Message queue
}

type RouteHandler func(conn *ServerConn, seq uint64, buf *buffer.Bytes) error
