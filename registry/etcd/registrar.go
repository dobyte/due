/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/17 1:22 上午
 * @Desc: TODO
 */

package etcd

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type registrar struct {
	registry *Registry
	cancel   context.CancelFunc
	kv       clientv3.KV
	lease    clientv3.Lease
	leaseID  clientv3.LeaseID
	mu       sync.Mutex
	stopped  atomic.Bool
	wg       sync.WaitGroup
}

func newRegistrar(registry *Registry) *registrar {
	r := &registrar{}
	r.kv = clientv3.NewKV(registry.opts.client)
	r.lease = clientv3.NewLease(registry.opts.client)
	r.registry = registry

	return r
}

// 注册服务
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
		r.lease.Revoke(ctx, leaseID)
		r.mu.Unlock()
		return errors.ErrIllegalOperation
	}

	if r.cancel != nil {
		r.cancel()
	}

	if r.leaseID != 0 {
		r.lease.Revoke(ctx, r.leaseID)
	}

	ctx, cancel := context.WithCancel(r.registry.ctx)
	r.cancel = cancel
	r.leaseID = leaseID

	r.wg.Add(1)
	go r.keepalive(ctx, leaseID, key, value)
	r.mu.Unlock()

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	defer r.stop()

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)

	_, err := r.kv.Delete(ctx, key)
	return err
}

func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()

	r.wg.Wait()

	r.mu.Lock()
	if r.leaseID != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), r.registry.opts.timeout)
		r.lease.Revoke(ctx, r.leaseID)
		cancel()
		r.leaseID = 0
	}
	r.mu.Unlock()

	if r.lease != nil {
		r.lease.Close()
	}
}

// 写入KV
func (r *registrar) put(ctx context.Context, key, value string) (clientv3.LeaseID, error) {
	res, err := r.lease.Grant(ctx, int64(r.registry.opts.leaseTTL.Seconds()))
	if err != nil {
		return 0, err
	}

	if _, err = r.kv.Put(ctx, key, value, clientv3.WithLease(res.ID)); err != nil {
		r.lease.Revoke(ctx, res.ID)
		return 0, err
	}

	return res.ID, nil
}

// 保活
func (r *registrar) keepalive(ctx context.Context, leaseID clientv3.LeaseID, key, value string) {
	defer r.wg.Done()

	chKA, err := r.lease.KeepAlive(ctx, leaseID)
	ok := err == nil

	for {
		if !ok {
			for i := 0; i < r.registry.opts.retryTimes; i++ {
				if ctx.Err() != nil {
					return
				}

				tctx, tcancel := context.WithTimeout(ctx, r.registry.opts.timeout)
				newLeaseID, err := r.put(tctx, key, value)
				tcancel()
				if err != nil {
					select {
					case <-time.After(r.registry.opts.retryInterval):
					case <-ctx.Done():
						return
					}
					continue
				}

				r.mu.Lock()
				if r.stopped.Load() {
					r.lease.Revoke(ctx, newLeaseID)
					r.mu.Unlock()
					return
				}
				if r.leaseID != 0 {
					r.lease.Revoke(ctx, r.leaseID)
				}
				r.leaseID = newLeaseID
				leaseID = newLeaseID
				r.mu.Unlock()

				chKA, err = r.lease.KeepAlive(ctx, newLeaseID)
				if err != nil {
					select {
					case <-time.After(r.registry.opts.retryInterval):
					case <-ctx.Done():
						return
					}
					continue
				}

				ok = true
				break
			}

			if !ok {
				return
			}
		}

		select {
		case _, ok = <-chKA:
			if !ok {
				if ctx.Err() != nil {
					return
				}
				continue
			}
		case <-ctx.Done():
			return
		}
	}
}
