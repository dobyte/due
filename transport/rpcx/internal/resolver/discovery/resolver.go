package discovery

import (
	"sync"
	"time"

	"github.com/dobyte/due/v2/log"
	cli "github.com/smallnest/rpcx/client"
)

// Resolver 服务发现模式服务发现器
// 维护服务地址列表，并支持订阅者接收地址变更通知
type Resolver struct {
	builder *Builder
	name    string
	filter  cli.ServiceDiscoveryFilter
	prw     sync.RWMutex
	pairs   []*cli.KVPair
	crw     sync.RWMutex
	chans   []chan []*cli.KVPair
	closed  bool
}

// newResolver 新建服务发现模式服务发现器
// @param name string 服务名
// @param builder *Builder 所属构建器
// @return @1 *Resolver 服务发现器实例
func newResolver(name string, builder *Builder) *Resolver {
	return &Resolver{
		name:    name,
		builder: builder,
	}
}

// GetServices 获取服务地址列表
// @return @1 []*cli.KVPair 服务地址列表
func (r *Resolver) GetServices() []*cli.KVPair {
	r.prw.RLock()
	defer r.prw.RUnlock()

	return r.pairs
}

// WatchService 监听服务地址变更
// 返回带缓冲的变更通知通道，已关闭时立即返回已关闭的通道
// @return @1 chan []*cli.KVPair 变更通知通道
func (r *Resolver) WatchService() chan []*cli.KVPair {
	ch := make(chan []*cli.KVPair, 10)

	r.crw.Lock()
	if r.closed {
		r.crw.Unlock()
		close(ch)
		return ch
	}
	r.chans = append(r.chans, ch)
	r.crw.Unlock()

	return ch
}

// RemoveWatcher 移除监听通道
// @param ch chan []*cli.KVPair 待移除的通道
func (r *Resolver) RemoveWatcher(ch chan []*cli.KVPair) {
	r.crw.Lock()
	defer r.crw.Unlock()

	i := -1

	for _, c := range r.chans {
		if c == ch {
			close(c)
		} else {
			i++
			r.chans[i] = c
		}
	}

	r.chans = r.chans[:i+1]
}

// Clone 克隆服务发现器
// 直接复用当前实例
// @param servicePath string 服务路径
// @return @1 cli.ServiceDiscovery 服务发现器
// @return @2 error 错误信息
func (r *Resolver) Clone(servicePath string) (cli.ServiceDiscovery, error) {
	return r, nil
}

// SetFilter 设置服务过滤函数
// @param filter cli.ServiceDiscoveryFilter 过滤函数
func (r *Resolver) SetFilter(filter cli.ServiceDiscoveryFilter) {
	r.filter = filter
}

// Close 关闭服务发现器
// 从构建器移除自身并关闭全部监听通道
func (r *Resolver) Close() {
	r.builder.removeResolver(r)

	r.crw.Lock()
	if r.closed {
		r.crw.Unlock()
		return
	}
	r.closed = true
	for _, c := range r.chans {
		close(c)
	}
	r.chans = nil
	r.crw.Unlock()
}

// updateState 更新服务地址状态并广播变更
// @param list []*cli.KVPair 最新服务地址列表
func (r *Resolver) updateState(list []*cli.KVPair) {
	var pairs []*cli.KVPair

	if r.filter != nil {
		pairs = make([]*cli.KVPair, 0, len(list))
		for _, pair := range list {
			if r.filter(pair) {
				pairs = append(pairs, pair)
			}
		}
	} else {
		pairs = list
	}

	r.prw.Lock()
	r.pairs = pairs
	r.prw.Unlock()

	r.crw.RLock()
	defer r.crw.RUnlock()

	if r.closed {
		return
	}

	for _, ch := range r.chans {
		select {
		case ch <- pairs:
			// 快速路径：消费方及时读，不产生协程
		default:
			// 慢路径：通道已满，最多等 1 分钟后丢弃
			go func(ch chan []*cli.KVPair) {
				defer func() { recover() }()

				select {
				case ch <- pairs:
				case <-time.After(time.Minute):
					log.Warn("chan is full and new change has been dropped")
				}
			}(ch)
		}
	}
}
