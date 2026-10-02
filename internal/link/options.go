package link

import (
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
)

type Options struct {
	ID                  string            // Instance ID
	Kind                cluster.Kind      // Instance kind
	Codec               encoding.Codec    // Codec
	Locator             locate.Locator    // Locator
	Registry            registry.Registry // Registry
	Encryptor           crypto.Encryptor  // Encryptor
	Dispatch            cluster.Dispatch  // Dispatch strategy for stateless routed messages
	ConnNum             int               // Number of connections
	CallTimeout         time.Duration     // Call timeout
	DialTimeout         time.Duration     // Dial timeout
	DialRetryTimes      int               // Number of dial retries
	FaultRecoveryTime   time.Duration     // Fault recovery time
	CommandQueueSize    int32             // Command queue size
	CommandWriteTimeout time.Duration     // Command write timeout
	WaitHandler         func() bool       // Wait handler
	DoneHandler         func() bool       // Done handler
}
