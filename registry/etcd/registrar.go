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
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type registrar struct {
	registry *Registry
	insID    string
	ctx      context.Context
	cancel   context.CancelFunc
	kv       clientv3.KV
	lease    clientv3.Lease
	leaseID  clientv3.LeaseID
	mu       sync.Mutex
	stopped  atomic.Bool
	wg       sync.WaitGroup
}

func newRegistrar(registry *Registry, insID string) *registrar {
	r := &registrar{}
	r.kv = clientv3.NewKV(registry.opts.client)
	r.insID = insID
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
		r.mu.Unlock()
		r.revoke(leaseID)
		return errors.ErrIllegalOperation
	}

	oldCancel := r.cancel
	oldLeaseID := r.leaseID
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.leaseID = leaseID
	r.wg.Add(1)
	r.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	if oldLeaseID != 0 {
		r.revoke(oldLeaseID)
	}

	go r.keepalive(r.ctx, leaseID, key, value)

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	defer r.stop()

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)

	if _, err := r.kv.Delete(ctx, key); err != nil {
		return err
	}

	return nil
}

// 停止注册
func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	r.cleanup()

	r.wg.Wait()
}

// 资源清理（不 wg.Wait，调用者需自行处理 goroutine 等待）
func (r *registrar) cleanup() {
	r.mu.Lock()
	cancel := r.cancel
	leaseID := r.leaseID
	r.cancel = nil
	r.leaseID = 0
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if leaseID != 0 {
		r.revoke(leaseID)
	}

	if r.lease != nil {
		if err := r.lease.Close(); err != nil {
			log.Warnf("close lease failed: %v", err)
		}
	}

	r.registry.registrars.Delete(r.insID)
}

// 写入KV
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

// 撤销租约
func (r *registrar) revoke(leaseID clientv3.LeaseID) {
	ctx, cancel := context.WithTimeout(context.Background(), r.registry.opts.timeout)
	defer cancel()

	if _, err := r.lease.Revoke(ctx, leaseID); err != nil {
		log.Warnf("revoke lease %d failed: %v", leaseID, err)
	}
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
					case <-time.After(backoff(i)):
						log.Warnf("etcd put kv failed, retry %d times, err: %v", i+1, err)
					case <-ctx.Done():
						return
					}
				} else {
					if chKA, err = r.lease.KeepAlive(ctx, newLeaseID); err != nil {
						r.revoke(newLeaseID)

						select {
						case <-time.After(backoff(i)):
							log.Warnf("etcd keepalive failed, retry %d times, err: %v", i+1, err)
						case <-ctx.Done():
							return
						}
					} else {
						r.mu.Lock()
						if r.stopped.Load() {
							r.mu.Unlock()
							r.revoke(newLeaseID)
							return
						}

						oldLeaseID := r.leaseID
						r.leaseID = newLeaseID
						r.mu.Unlock()

						if oldLeaseID != 0 {
							r.revoke(oldLeaseID)
						}

						ok = true
						break
					}
				}
			}

			if !ok {
				if !r.stopped.CompareAndSwap(false, true) {
					return
				}

				r.cleanup()

				log.Errorf("etcd keepalive failed after %d retries, service registration lost", r.registry.opts.retryTimes)
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
