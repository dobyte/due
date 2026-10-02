package ch

import (
	"context"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

// virtualNodes is the number of virtual nodes per sub-connection on the hash ring.
const virtualNodes = 150

var _ balancer.Balancer = &Balancer{}

// Balancer is a consistent hash load balancer.
//
// It maintains a hash ring so that requests with the same key are always routed to the same
// sub-connection.
type Balancer struct {
	cc       balancer.ClientConn
	opts     balancer.BuildOptions
	subConns map[balancer.SubConn]resolver.Address
	scStates map[balancer.SubConn]connectivity.State
	cse      balancer.ConnectivityStateEvaluator
	picker   balancer.Picker
	mu       sync.Mutex
}

// UpdateClientConnState updates the client connection state.
//
// It synchronizes the address list pushed by the resolver, removes stale sub-connections and
// creates missing ones.
func (b *Balancer) UpdateClientConnState(s balancer.ClientConnState) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	addrsSet := make(map[string]struct{})
	for _, addr := range s.ResolverState.Addresses {
		addrsSet[addr.Addr] = struct{}{}
	}

	for sc, addr := range b.subConns {
		if _, ok := addrsSet[addr.Addr]; !ok {
			b.cse.RecordTransition(b.scStates[sc], connectivity.Shutdown)
			sc.Shutdown()
			delete(b.subConns, sc)
			delete(b.scStates, sc)
		}
	}

	for _, addr := range s.ResolverState.Addresses {
		exists := false
		for _, existingAddr := range b.subConns {
			if existingAddr.Addr == addr.Addr {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		sc, err := b.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{})
		if err != nil {
			continue
		}

		b.subConns[sc] = addr
		b.scStates[sc] = connectivity.Idle
		b.cse.RecordTransition(connectivity.Shutdown, connectivity.Idle)
		sc.Connect()
	}

	b.updatePicker()

	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.cse.CurrentState(),
		Picker:            b.picker,
	})

	return nil
}

func (b *Balancer) updatePicker() {
	if len(b.subConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	readyConns := make([]*hashSubConn, 0, len(b.subConns))
	for sc, addr := range b.subConns {
		if b.scStates[sc] == connectivity.Ready {
			readyConns = append(readyConns, &hashSubConn{sc: sc, key: addr.Addr})
		}
	}

	if len(readyConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	b.picker = &Picker{ring: newConsistentRing(readyConns)}
}

func (b *Balancer) ResolverError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.picker = &Picker{err: err}
	b.cc.UpdateState(balancer.State{
		ConnectivityState: connectivity.TransientFailure,
		Picker:            b.picker,
	})
}

func (b *Balancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subConns[sc]; !ok {
		return
	}

	oldState := b.scStates[sc]
	b.scStates[sc] = state.ConnectivityState

	switch state.ConnectivityState {
	case connectivity.Idle:
		sc.Connect()
	case connectivity.Shutdown:
		delete(b.subConns, sc)
		delete(b.scStates, sc)
	}

	if oldState != state.ConnectivityState {
		b.cse.RecordTransition(oldState, state.ConnectivityState)
	}

	b.updatePicker()

	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.cse.CurrentState(),
		Picker:            b.picker,
	})
}

func (b *Balancer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sc := range b.subConns {
		sc.Shutdown()
	}

	b.subConns = nil
	b.scStates = nil
	b.picker = nil
}

// ExitIdle leaves the idle state and triggers a connection for every idle sub-connection.
func (b *Balancer) ExitIdle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sc, state := range b.scStates {
		if state == connectivity.Idle {
			sc.Connect()
		}
	}
}

// hashSubConn is a sub-connection to be hashed.
type hashSubConn struct {
	sc  balancer.SubConn
	key string // Hash node identifier (service address)
}

// ringNode is a node on the hash ring.
type ringNode struct {
	hash uint32
	sc   balancer.SubConn
}

// consistentRing is a consistent hash ring sorted by hash value in ascending order.
type consistentRing struct {
	nodes []ringNode
}

// hashKey computes the hash of a string; FNV-1a offers better distribution.
func hashKey(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// newConsistentRing builds a hash ring, generating several virtual nodes per sub-connection to
// balance distribution.
func newConsistentRing(subConns []*hashSubConn) *consistentRing {
	nodes := make([]ringNode, 0, len(subConns)*virtualNodes)
	for _, sc := range subConns {
		for i := 0; i < virtualNodes; i++ {
			key := sc.key + "#" + strconv.Itoa(i)
			nodes = append(nodes, ringNode{hash: hashKey(key), sc: sc.sc})
		}
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].hash < nodes[j].hash })

	return &consistentRing{nodes: nodes}
}

// get returns the sub-connection for the given hash key.
func (r *consistentRing) get(key string) (balancer.SubConn, bool) {
	if len(r.nodes) == 0 {
		return nil, false
	}

	idx := sort.Search(len(r.nodes), func(i int) bool { return r.nodes[i].hash >= hashKey(key) })
	if idx == len(r.nodes) {
		idx = 0
	}

	return r.nodes[idx].sc, true
}

// hashKeyCtxKey is the context key for the consistent hash key.
type hashKeyCtxKey struct{}

// WithHashKey injects the consistent hash key into the context; when unset, the RPC method name is
// used as the hash key by default.
func WithHashKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, hashKeyCtxKey{}, key)
}

func getHashKey(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if key, ok := ctx.Value(hashKeyCtxKey{}).(string); ok {
		return key
	}

	return ""
}

// Picker is the consistent hash picker that selects a target sub-connection based on the hash ring.
type Picker struct {
	ring *consistentRing
	err  error
}

var _ balancer.Picker = &Picker{}

// Pick selects a target sub-connection.
//
// It routes by the hash key in the request context and falls back to the RPC method name when none
// is specified.
func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if p.err != nil {
		return balancer.PickResult{}, p.err
	}

	key := getHashKey(info.Ctx)
	if key == "" {
		key = info.FullMethodName
	}

	if sc, ok := p.ring.get(key); ok {
		return balancer.PickResult{SubConn: sc}, nil
	}

	return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
}
