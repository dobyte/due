package random

import (
	"math/rand"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

var _ balancer.Balancer = &Balancer{}

// Balancer is a random load balancer.
//
// It picks a ready sub-connection at random to handle a request.
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

	readyConns := make([]balancer.SubConn, 0, len(b.subConns))
	for sc, state := range b.scStates {
		if state == connectivity.Ready {
			readyConns = append(readyConns, sc)
		}
	}

	if len(readyConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	b.picker = &Picker{subConns: readyConns}
}

// ResolverError handles a resolver error.
//
// It passes the error to the picker and updates the connection state to transient failure.
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

// Picker is the random picker that selects one sub-connection from the list at random.
type Picker struct {
	subConns []balancer.SubConn
	err      error
}

var _ balancer.Picker = &Picker{}

// Pick selects a target sub-connection.
//
// It returns the only sub-connection directly when there is one, and picks at random otherwise.
func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if p.err != nil {
		return balancer.PickResult{}, p.err
	}

	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	if len(p.subConns) == 1 {
		return balancer.PickResult{SubConn: p.subConns[0]}, nil
	}

	idx := rand.Intn(len(p.subConns))

	return balancer.PickResult{SubConn: p.subConns[idx]}, nil
}
