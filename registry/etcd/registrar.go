package etcd

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// registrar is a service registrar.
//
// It registers and deregisters service instances and keeps the lease bound to the service key
// alive through a keepalive goroutine.
type registrar struct {
	registry *Registry          // Service registry
	insID    string             // Service instance ID
	ctx      context.Context    // Keepalive context
	cancel   context.CancelFunc // Keepalive cancel function
	kv       clientv3.KV        // KV client
	lease    clientv3.Lease     // Lease client
	leaseID  clientv3.LeaseID   // Currently effective lease ID
	mu       sync.Mutex         // Guards ctx, cancel and leaseID
	stopped  atomic.Bool        // Whether the registrar has stopped
	wg       sync.WaitGroup     // Waits for the keepalive goroutine to exit
}

// newRegistrar returns a new service registrar for the given registry and instance ID.
func newRegistrar(registry *Registry, insID string) *registrar {
	r := &registrar{}
	r.kv = clientv3.NewKV(registry.opts.client)
	r.insID = insID
	r.lease = clientv3.NewLease(registry.opts.client)
	r.registry = registry

	return r
}

// register registers a service instance.
//
// It serializes the instance, writes it to etcd with a bound lease and then starts a keepalive
// goroutine for that lease. Registering the same instance again revokes the previous lease and
// rebuilds the keepalive stream.
func (r *registrar) register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.stopped.Load() {
		return errors.ErrIllegalOperation
	}

	value, err := marshal(ins)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)

	leaseID, err := r.put(ctx, key, value)
	if err != nil {
		return err
	}

	r.mu.Lock()

	if r.stopped.Load() {
		r.mu.Unlock()
		r.revoke(leaseID)
		return errors.ErrIllegalOperation
	}

	oldCancel := r.cancel
	oldLeaseID := r.leaseID
	keepaliveCtx, cancel := context.WithCancel(r.registry.ctx)
	r.ctx, r.cancel = keepaliveCtx, cancel
	r.leaseID = leaseID
	r.wg.Add(1)
	r.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	if oldLeaseID != 0 {
		r.revoke(oldLeaseID)
	}

	go r.keepalive(keepaliveCtx, leaseID, key, value)

	return nil
}

// deregister deregisters a service instance.
//
// It deletes the service key explicitly first. A failed delete is not fatal: stop revokes the
// current lease, which reclaims the key as a fallback, so no misleading failure error is returned
// to the caller.
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	defer r.stop()

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)

	if _, err := r.kv.Delete(ctx, key); err != nil {
		log.Warnf("etcd deregister delete %s failed, will revoke lease instead: %v", key, err)
	}

	return nil
}

// stop stops the registrar.
//
// It removes the registrar from the registry, cancels the keepalive context and waits for the
// keepalive goroutine to exit, then revokes the last effective lease and closes the lease client.
// It is idempotent and returns immediately on repeated calls.
func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	// Remove the registrar from the registry first so that a concurrent re-registration does not
	// hit an already stopped registrar.
	r.registry.registrars.Delete(r.insID)

	// Cancel the keepalive context and wait for the keepalive goroutine to exit before releasing
	// resources, so that no in-flight network operation outlives the closed lease client.
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	r.wg.Wait()

	// Revoke the last effective lease. The keepalive goroutine has exited and no new lease can be
	// committed, so the value read here is the final lease.
	r.mu.Lock()
	leaseID := r.leaseID
	r.leaseID = 0
	r.mu.Unlock()

	if leaseID != 0 {
		r.revoke(leaseID)
	}

	if r.lease != nil {
		if err := r.lease.Close(); err != nil {
			log.Warnf("close lease failed: %v", err)
		}
	}
}

// put writes the key-value pair to etcd and returns the newly granted lease ID.
func (r *registrar) put(ctx context.Context, key, value string) (clientv3.LeaseID, error) {
	res, err := r.lease.Grant(ctx, int64(r.registry.opts.leaseTTL.Seconds()))
	if err != nil {
		return 0, err
	}

	if _, err = r.kv.Put(ctx, key, value, clientv3.WithLease(res.ID)); err != nil {
		r.revoke(res.ID)
		return 0, err
	}

	return res.ID, nil
}

// revoke revokes leaseID.
//
// It revokes with a dedicated timeout context so that an etcd failure does not block for a long
// time.
func (r *registrar) revoke(leaseID clientv3.LeaseID) {
	ctx, cancel := context.WithTimeout(context.Background(), r.registry.opts.timeout)
	defer cancel()

	if _, err := r.lease.Revoke(ctx, leaseID); err != nil {
		log.Warnf("revoke lease %d failed: %v", leaseID, err)
	}
}

// keepalive keeps the service lease alive.
//
// When the keepalive stream breaks, it re-registers the service and rebuilds the stream instead of
// deregistering it on a transient network failure. The rebuild backoff (minRetryDelay to
// maxRetryDelay) is maintained across rounds by this goroutine: the backoff is only reset when the
// previous stream stayed alive for at least stableDuration, which prevents a frequently dying
// stream from repeatedly resetting the backoff. It exits only when the registration is stopped or
// superseded.
func (r *registrar) keepalive(ctx context.Context, leaseID clientv3.LeaseID, key, value string) {
	defer r.wg.Done()

	var (
		ok    bool
		delay = minRetryDelay
		start time.Time // Establishment time of the current keepalive stream, used for the stability check
	)

	chKA, err := r.lease.KeepAlive(ctx, leaseID)
	ok = err == nil
	if ok {
		// The first keepalive stream was established successfully; record the time so that the
		// stability of this stream can be evaluated when it breaks.
		start = time.Now()
	}

	for {
		if !ok {
			// The keepalive stream broke; re-register and rebuild the stream.
			if chKA, ok, delay = r.renew(ctx, key, value, delay); !ok {
				return
			}
			start = time.Now()
			continue
		}

		select {
		case _, ok = <-chKA:
			if !ok {
				if ctx.Err() != nil {
					return
				}

				// Adjust the rebuild backoff according to how long the previous keepalive stream
				// lived:
				// - Alive for at least stableDuration: the link is healthy, so reset the backoff to
				//   recover quickly.
				// - Died shortly after being established: the link is unstable, so use exponential
				//   backoff to avoid busy spinning.
				if time.Since(start) >= stableDuration {
					delay = minRetryDelay
				} else {
					delay = min(delay*2, maxRetryDelay)
				}
				continue
			}
		case <-ctx.Done():
			return
		}
	}
}

// renew re-registers the service and rebuilds the keepalive stream.
//
// It retries with exponential backoff capped at maxRetryDelay until the re-registration succeeds or
// the current registration is stopped or superseded. The starting delay is passed in by the caller
// and the latest interval is returned, so that the backoff state carries across rounds instead of
// being lost when keepalive streams frequently die early.
func (r *registrar) renew(ctx context.Context, key, value string, delay time.Duration) (<-chan *clientv3.LeaseKeepAliveResponse, bool, time.Duration) {
	var chKA <-chan *clientv3.LeaseKeepAliveResponse

	for {
		if r.stopped.Load() {
			return nil, false, delay
		}

		select {
		case <-ctx.Done():
			return nil, false, delay
		case <-time.After(delay):
		}

		// Confirm before putting that the registration is still maintained by this goroutine, so
		// that a late write does not overwrite the key of a new registration after the current one
		// was stopped or superseded while waiting for the backoff.
		r.mu.Lock()
		if r.stopped.Load() || r.ctx != ctx {
			r.mu.Unlock()
			return nil, false, delay
		}
		r.mu.Unlock()

		tctx, tcancel := context.WithTimeout(ctx, r.registry.opts.timeout)
		newLeaseID, err := r.put(tctx, key, value)
		tcancel()
		if err == nil {
			if chKA, err = r.lease.KeepAlive(ctx, newLeaseID); err != nil {
				r.revoke(newLeaseID)
			}
		}

		if err != nil {
			delay = min(delay*2, maxRetryDelay)
			log.Warnf("etcd re-register failed, will retry after %v, err: %v", delay, err)
			continue
		}

		// Re-registration succeeded; make sure the registration is still maintained by this
		// goroutine before committing the new lease.
		r.mu.Lock()
		if r.stopped.Load() || r.ctx != ctx {
			r.mu.Unlock()
			r.revoke(newLeaseID)
			return nil, false, delay
		}

		oldLeaseID := r.leaseID
		r.leaseID = newLeaseID
		r.mu.Unlock()

		if oldLeaseID != 0 {
			r.revoke(oldLeaseID)
		}

		return chKA, true, delay
	}
}
