package drpc

import (
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/bridge"
)

type ServerOptions struct {
	DisableLocal   bool              // Disable local provider registration in the cluster wrapper
	DisableRemote  bool              // Disable the cluster wrapper's TCP listener
	Directory      *bridge.Directory // Optional isolated process-local directory
	Addr           string            // Listen address
	Expose         bool              // Whether to expose the public IP
	WriteTimeout   time.Duration     // Write timeout
	WriteQueueSize int32             // Write queue size
}

type ClientOptions struct {
	DisableLocal      bool              // Force remote delivery in the cluster wrapper
	Directory         *bridge.Directory // Optional isolated process-local directory
	ID                string            // Instance ID
	Kind              cluster.Kind      // Instance kind
	ConnNum           int               // Number of connections
	CallTimeout       time.Duration     // Call timeout
	DialTimeout       time.Duration     // Dial timeout
	DialRetryTimes    int               // Number of dial retries
	WriteTimeout      time.Duration     // Write timeout
	WriteQueueSize    int32             // Write queue size
	FaultRecoveryTime time.Duration     // Fault recovery time
}
