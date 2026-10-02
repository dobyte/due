package wrr

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

// WeightAttrKey is the attribute key for the service instance weight, attached to the address by
// the resolver.
const WeightAttrKey = "weight"

var _ balancer.Balancer = &Balancer{}

// Balancer is a weighted round-robin load balancer.
//
// It distributes requests among ready sub-connections by weight using the smooth weighted
// round-robin algorithm.
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

	readyConns := make([]*weightedSubConn, 0, len(b.subConns))
	for sc, addr := range b.subConns {
		if b.scStates[sc] == connectivity.Ready {
			readyConns = append(readyConns, &weightedSubConn{
				sc:     sc,
				weight: getWeight(addr),
			})
		}
	}

	if len(readyConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	b.picker = &Picker{subConns: readyConns}
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

// UpdateSubConnState updates the sub-connection state.
//
// An idle connection triggers a reconnect, a shutdown connection is removed from the map, and the
// picker is updated accordingly.
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

// Close closes the load balancer, shuts down all sub-connections and clears its state.
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

// getWeight reads the service instance weight from the address attributes, defaulting to 1 when it
// is unset or missing.
func getWeight(addr resolver.Address) int {
	if addr.Attributes != nil {
		if v, ok := addr.Attributes.Value(WeightAttrKey).(uint32); ok && v > 0 {
			return int(v)
		}
	}
	return 1
}

// weightedSubConn is a weighted sub-connection that keeps the weight state required by the smooth
// weighted round-robin algorithm.
type weightedSubConn struct {
	sc            balancer.SubConn
	weight        int // Static weight
	currentWeight int // Smooth weighted round-robin dynamic weight
}

// Picker is the weighted round-robin picker.
type Picker struct {
	subConns []*weightedSubConn
	err      error
	mu       sync.Mutex
}

var _ balancer.Picker = &Picker{}

// Pick selects a target sub-connection.
//
// It uses the smooth weighted round-robin algorithm and each time selects the sub-connection with
// the largest current dynamic weight.
func (p *Picker) Pick(_ balancer.PickInfo) (balancer.PickResult, error) {
	if p.err != nil {
		return balancer.PickResult{}, p.err
	}

	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Smooth weighted round-robin: select the sub-connection with the largest current dynamic
	// weight each time.
	var (
		best  *weightedSubConn
		total int
	)
	for _, sc := range p.subConns {
		sc.currentWeight += sc.weight
		total += sc.weight
		if best == nil || sc.currentWeight > best.currentWeight {
			best = sc
		}
	}
	best.currentWeight -= total

	return balancer.PickResult{SubConn: best.sc}, nil
}
