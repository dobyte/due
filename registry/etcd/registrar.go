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
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/dobyte/due/registry"
)

type registrar struct {
	registry *Registry
	ctx      context.Context
	cancel   context.CancelFunc
	kv       clientv3.KV
	lease    clientv3.Lease
}

func newRegistrar(registry *Registry) *registrar {
	r := &registrar{}
	r.kv = clientv3.NewKV(registry.client)
	r.lease = clientv3.NewLease(registry.client)
	r.ctx, r.cancel = context.WithCancel(registry.ctx)
	r.registry = registry

	return r
}

// 注册服务
func (r *registrar) register(ctx context.Context, ins *registry.ServiceInstance) error {
	value, err := marshal(ins)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)

	leaseID, err := r.put(ctx, key, value)
	if err != nil {
		return err
	}

	go r.heartbeat(leaseID, key, value)

	return nil
}

// 解注册服务
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) (err error) {
	r.cancel()

	r.registry.registrars.Delete(ins.ID)

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)
	_, err = r.kv.Delete(ctx, key)

	return
}

// 写入KV
func (r *registrar) put(ctx context.Context, key, value string) (clientv3.LeaseID, error) {
	res, err := r.lease.Grant(ctx, 5)
	if err != nil {
		return 0, err
	}

	_, err = r.kv.Put(ctx, key, value, clientv3.WithLease(res.ID))
	if err != nil {
		return 0, err
	}

	return res.ID, nil
}

// 心跳
func (r *registrar) heartbeat(leaseID clientv3.LeaseID, key, value string) {
	chKA, err := r.lease.KeepAlive(r.ctx, leaseID)
	ok := err == nil

	for {
		if !ok {
			for i := 0; i < r.registry.opts.retryTimes; i++ {
				if r.ctx.Err() != nil {
					return
				}

				time.Sleep(r.registry.opts.retryInterval)

				ctx, cancel := context.WithTimeout(r.ctx, r.registry.opts.timeout)
				leaseID, err = r.put(ctx, key, value)
				cancel()
				if err != nil {
					continue
				}

				chKA, err = r.lease.KeepAlive(r.ctx, leaseID)
				if err != nil {
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
				if r.ctx.Err() != nil {
					return
				}
				continue
			}
		case <-r.ctx.Done():
			return
		}
	}
}
