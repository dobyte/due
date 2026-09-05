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

// registrar 服务注册器
// 负责服务实例的注册与解注册，并通过保活协程维持服务键绑定的租约
type registrar struct {
	registry *Registry          // 服务注册中心
	insID    string             // 服务实例ID
	ctx      context.Context    // 保活上下文
	cancel   context.CancelFunc // 保活取消函数
	kv       clientv3.KV        // KV客户端
	lease    clientv3.Lease     // 租约客户端
	leaseID  clientv3.LeaseID   // 当前生效的租约ID
	mu       sync.Mutex         // 保护 ctx/cancel/leaseID 的互斥锁
	stopped  atomic.Bool        // 是否已停止
	wg       sync.WaitGroup     // 等待保活协程退出
}

// 构建服务注册器
// @param registry *Registry 服务注册中心
// @param insID string 服务实例ID
// @return @1 *registrar 服务注册器实例
func newRegistrar(registry *Registry, insID string) *registrar {
	r := &registrar{}
	r.kv = clientv3.NewKV(registry.opts.client)
	r.insID = insID
	r.lease = clientv3.NewLease(registry.opts.client)
	r.registry = registry

	return r
}

// 注册服务
// 将服务实例序列化后写入 etcd 并绑定租约，随后启动保活协程维持租约；
// 重复注册同一服务实例会撤销旧租约并重建保活流
// @param ctx context.Context 上下文
// @param ins *registry.ServiceInstance 服务实例
// @return @1 error 注册失败时返回的错误
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

// 解注册服务
// 先显式删除服务键；删除失败不致命——stop 会撤销当前租约（键随租约回收）作为兜底删除，
// 故不向调用方返回误导性的失败错误
// @param ctx context.Context 上下文
// @param ins *registry.ServiceInstance 服务实例
// @return @1 error 解注册失败时返回的错误
func (r *registrar) deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	defer r.stop()

	key := fmt.Sprintf("/%s/%s/%s", r.registry.opts.namespace, ins.Name, ins.ID)

	if _, err := r.kv.Delete(ctx, key); err != nil {
		log.Warnf("etcd deregister delete %s failed, will revoke lease instead: %v", key, err)
	}

	return nil
}

// 停止注册
// 依次完成：从注册表中移除注册器、取消保活上下文并等待保活协程退出、
// 撤销最后一次生效的租约并关闭租约客户端；幂等，重复调用直接返回
func (r *registrar) stop() {
	if !r.stopped.CompareAndSwap(false, true) {
		return
	}

	// 先将注册器从注册表中移除，避免并发重新注册时命中已停止的注册器
	r.registry.registrars.Delete(r.insID)

	// 取消保活上下文，等待保活协程退出后再释放资源，
	// 确保不会在租约客户端关闭后仍有进行中的网络操作
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	r.wg.Wait()

	// 撤销最后一次生效的租约（保活协程已退出，不再有新的租约提交，此时读取的即为最终租约）
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
// 以独立超时上下文执行撤销，避免因 etcd 异常导致长时间阻塞
// @param leaseID clientv3.LeaseID 待撤销的租约ID
func (r *registrar) revoke(leaseID clientv3.LeaseID) {
	ctx, cancel := context.WithTimeout(context.Background(), r.registry.opts.timeout)
	defer cancel()

	if _, err := r.lease.Revoke(ctx, leaseID); err != nil {
		log.Warnf("revoke lease %d failed: %v", leaseID, err)
	}
}

// 保活
// 保活流中断后自动重新注册并重建保活流，不会因瞬时网络故障主动注销服务；
// 重建退避（minRetryDelay ~ maxRetryDelay）由本协程跨轮次统一维护：
// 上一次保活流稳定存活超过 stableDuration 才允许重置退避，防止流频繁短命断开时退避被反复重置失效；
// 仅当注册被停止或取代时退出
// @param ctx context.Context 保活上下文
// @param leaseID clientv3.LeaseID 当前生效的租约ID
// @param key string 服务键
// @param value string 服务数据
func (r *registrar) keepalive(ctx context.Context, leaseID clientv3.LeaseID, key, value string) {
	defer r.wg.Done()

	var (
		ok    bool
		delay = minRetryDelay
		start time.Time // 当前保活流建立时刻，用于稳定判定
	)

	chKA, err := r.lease.KeepAlive(ctx, leaseID)
	ok = err == nil
	if ok {
		// 首次保活流建立成功，记录建立时刻，供该流断开时进行稳定判定
		start = time.Now()
	}

	for {
		if !ok {
			// 保活流中断，重新注册并重建保活流
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

				// 根据上一次保活流的存活时长调整重建退避：
				// - 稳定存活超过 stableDuration：链路健康，重置退避以便快速恢复；
				// - 短命即断开：链路不稳定，指数退避防空转
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

// 重新注册并重建保活流
// 采用指数退避（minRetryDelay ~ maxRetryDelay 封顶）持续重试，直至重新注册成功或当前注册被停止/取代；
// 起始退避由调用方传入并回传最新间隔，使退避状态跨轮次延续，避免保活流频繁短命断开时退避失效
// @param ctx context.Context 保活上下文（当前注册的保活上下文）
// @param key string 服务键
// @param value string 服务数据
// @param delay time.Duration 本次重建的起始等待间隔
// @return @1 <-chan *clientv3.LeaseKeepAliveResponse 保活响应通道，重新注册成功后返回
// @return @2 bool 是否重新注册成功；返回false表示本协程应退出
// @return @3 time.Duration 最新等待间隔（内部指数退避递增后的结果，供调用方跨轮次延续）
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

		// put 前再次确认注册仍由本协程维护，避免等待退避期间注册被停止/取代后，
		// 仍发出迟到的写入覆盖新注册的 key
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

		// 重新注册成功，确认注册仍由本协程维护后再提交新租约
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
