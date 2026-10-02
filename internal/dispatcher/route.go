package dispatcher

import (
	"math/rand/v2"
	"sync/atomic"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

type Route struct {
	eps1       []*serviceEndpoint                 // All endpoints, including instances in the work state
	eps2       []*serviceEndpoint                 // All endpoints, including instances in the busy state
	eps3       map[string]*serviceEndpoint        // All endpoints, including instances in the work, busy and hang states
	route      registry.Route                     // Route information
	group      string                             // Group the route belongs to
	counter    atomic.Uint64                      // Round-robin counter
	wrSeq      atomic.Pointer[[]*serviceEndpoint] // Precomputed smooth weighted round-robin sequence (built lazily)
	dispatcher *Dispatcher                        // Dispatcher
}

func newRoute(dispatcher *Dispatcher, group string, route registry.Route) *Route {
	r := &Route{
		eps1:       make([]*serviceEndpoint, 0),
		eps2:       make([]*serviceEndpoint, 0),
		eps3:       make(map[string]*serviceEndpoint),
		route:      route,
		group:      group,
		dispatcher: dispatcher,
	}

	return r
}

// ID returns the route ID.
func (r *Route) ID() int32 {
	return r.route.ID
}

// Group returns the group the route belongs to.
func (r *Route) Group() string {
	return r.group
}

// Internal reports whether the route is internal.
func (r *Route) Internal() bool {
	return r.route.Internal
}

// Stateful reports whether the route is stateful.
func (r *Route) Stateful() bool {
	return r.route.Stateful
}

// Authorized reports whether the route requires authorization.
func (r *Route) Authorized() bool {
	return r.route.Authorized
}

// FindEndpoint returns a service endpoint of the route. When insID is provided and non-empty, the
// endpoint is resolved directly; otherwise it is chosen according to the dispatcher's strategy.
func (r *Route) FindEndpoint(insID ...string) (*endpoint.Endpoint, error) {
	if len(insID) > 0 && insID[0] != "" {
		return r.directDispatch(insID[0])
	} else {
		switch r.dispatcher.dispatch {
		case cluster.RoundRobin:
			return r.roundRobinDispatch()
		case cluster.WeightedRoundRobin:
			return r.weightedRoundRobinDispatch()
		default:
			return r.randomDispatch()
		}
	}
}

// directDispatch resolves the endpoint of the instance identified by insID.
func (r *Route) directDispatch(insID string) (*endpoint.Endpoint, error) {
	sep, ok := r.eps3[insID]
	if !ok {
		return nil, errors.ErrNotFoundEndpoint
	}

	return sep.endpoint, nil
}

// randomDispatch picks an available endpoint at random.
func (r *Route) randomDispatch() (*endpoint.Endpoint, error) {
	if eps := r.loadAvailableEndpoints(); len(eps) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	} else {
		return eps[rand.IntN(len(eps))].endpoint, nil
	}
}

// roundRobinDispatch picks an available endpoint in round-robin order.
func (r *Route) roundRobinDispatch() (*endpoint.Endpoint, error) {
	if eps := r.loadAvailableEndpoints(); len(eps) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	} else {
		return eps[(r.counter.Add(1)-1)%uint64(len(eps))].endpoint, nil
	}
}

// weightedRoundRobinDispatch picks an available endpoint using smooth weighted round-robin.
//
// It selects endpoints from a precomputed smooth sequence through an atomic cursor, leaving the hot
// path lock-free. Because the selection sequence of smooth weighted round-robin (SWRR) is periodic,
// the precomputed distribution is exactly equivalent to the online one.
func (r *Route) weightedRoundRobinDispatch() (*endpoint.Endpoint, error) {
	eps := r.loadAvailableEndpoints()
	if len(eps) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	}

	seq := r.loadWRSequence(eps)

	return seq[(r.counter.Add(1)-1)%uint64(len(seq))].endpoint, nil
}

// loadWRSequence returns the smooth weighted round-robin sequence built from eps.
//
// The sequence is built lazily. Concurrent builds produce identical content, so a failed
// compare-and-swap simply reuses the stored value.
func (r *Route) loadWRSequence(eps []*serviceEndpoint) []*serviceEndpoint {
	if seq := r.wrSeq.Load(); seq != nil {
		return *seq
	}

	seq := buildSmoothWRSequence(eps)

	if r.wrSeq.CompareAndSwap(nil, &seq) {
		return seq
	}

	return *r.wrSeq.Load()
}

// buildSmoothWRSequence builds the smooth weighted round-robin sequence from eps.
//
// The selection sequence is generated offline with the smooth weighted round-robin algorithm, and
// the weights are reduced by their GCD to bound the sequence length. A weight that is unset (<= 0)
// falls back to the default weight 1, matching the behavior of the original online algorithm.
func buildSmoothWRSequence(eps []*serviceEndpoint) []*serviceEndpoint {
	weights := make([]int, len(eps))

	g := 0
	for i, se := range eps {
		weight := se.weight
		if weight <= 0 {
			weight = 1
		}
		weights[i] = weight
		g = gcd(g, weight)
	}

	var total int
	for i := range weights {
		weights[i] /= g
		total += weights[i]
	}

	var (
		curr = make([]int, len(eps))
		seq  = make([]*serviceEndpoint, 0, total)
	)

	for range total {
		selected := 0
		for i := range eps {
			curr[i] += weights[i]
			if curr[i] > curr[selected] {
				selected = i
			}
		}
		curr[selected] -= total
		seq = append(seq, eps[selected])
	}

	return seq
}

// gcd returns the greatest common divisor of a and b.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}

// loadAvailableEndpoints returns the endpoints currently eligible to serve traffic.
func (r *Route) loadAvailableEndpoints() []*serviceEndpoint {
	switch {
	case len(r.eps1) > 0:
		return r.eps1
	case len(r.eps2) > 0:
		return r.eps2
	default:
		return nil
	}
}

// addServiceEndpoint adds a service endpoint to the route according to the instance state.
func (r *Route) addServiceEndpoint(se *serviceEndpoint) {
	switch se.state {
	case cluster.Work.String():
		r.eps1 = append(r.eps1, se)
	case cluster.Busy.String():
		r.eps2 = append(r.eps2, se)
	case cluster.Hang.String():
		// ignore
	default:
		return
	}

	r.eps3[se.insID] = se
}
