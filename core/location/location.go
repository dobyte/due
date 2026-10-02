package location

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
)

// Result is the geolocation result of an IP address.
type Result struct {
	IP       string `json:"ip"`       // IP address
	Country  string `json:"country"`  // Country
	Province string `json:"province"` // Province, autonomous region or municipality
	City     string `json:"city"`     // City
	ISP      string `json:"isp"`      // ISP
}

// Location is a geolocation resolver that runs several resolvers concurrently to resolve an IP
// address.
type Location struct {
	resolvers []Resolver // Resolver list
}

// NewLocation returns a new Location backed by the given resolvers.
func NewLocation(resolvers ...Resolver) *Location {
	return &Location{
		resolvers: resolvers,
	}
}

// Parse resolves the geolocation information of ip.
//
// It returns as soon as one resolver succeeds, or an error when every resolver fails or ctx is
// done.
func (l *Location) Parse(ctx context.Context, ip string) (*Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		ch    = make(chan *Result, len(l.resolvers))
		done  = make(chan struct{})
		wg    sync.WaitGroup
		state atomic.Bool
	)

	for _, r := range l.resolvers {
		wg.Go(func() {
			loc, err := r.Resolve(ctx, ip)
			if err != nil {
				return
			}

			if state.CompareAndSwap(false, true) {
				ch <- loc
			}
		})
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case loc := <-ch:
		return loc, nil
	case <-done:
		select {
		case loc := <-ch:
			return loc, nil
		default:
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, errors.ErrNotFoundIPAddress
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
