package drpc

import (
	"time"

	"github.com/dobyte/due/v2/cluster"
)

type ServerOptions struct {
	Addr           string        // Listen address
	Expose         bool          // Whether to expose the public IP
	WriteTimeout   time.Duration // Write timeout
	WriteQueueSize int32         // Write queue size
}

type ClientOptions struct {
	ID                string        // Instance ID
	Kind              cluster.Kind  // Instance kind
	ConnNum           int           // Number of connections
	CallTimeout       time.Duration // Call timeout
	DialTimeout       time.Duration // Dial timeout
	DialRetryTimes    int           // Number of dial retries
	WriteTimeout      time.Duration // Write timeout
	WriteQueueSize    int32         // Write queue size
	FaultRecoveryTime time.Duration // Fault recovery time
}
