package def

// Dispatch is the message dispatch strategy for stateless routing.
type Dispatch string

const (
	Random             Dispatch = "random" // Random dispatch
	RoundRobin         Dispatch = "rr"     // Round-robin dispatch
	WeightedRoundRobin Dispatch = "wrr"    // Weighted round-robin dispatch
	ConsistentHash     Dispatch = "ch"     // Consistent-hash dispatch
)
