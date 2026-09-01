package etcd

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// 监听器
type watcher struct {
	ctx     context.Context                  // 上下文
	cancel  context.CancelFunc               // 取消函数
	source  *Source                          // 配置源
	watcher clientv3.Watcher                 // etcd监听器
	watchCh clientv3.WatchChan               // etcd监听通道
	mu      sync.Mutex                       // 发送锁
	chWatch chan []*config.Configuration     // 配置变更通道
	rw      sync.RWMutex                     // 配置快照读写锁
	configs map[string]*config.Configuration // 配置快照
	stopped atomic.Bool                      // 是否已停止
	wg      sync.WaitGroup                   // 等待协程退出
}

// 创建监听器
// 以全量拉取结果作为初始快照，并从拉取时的版本号之后开始监听，避免丢失配置变更
// @param ctx context.Context 上下文
// @param s *Source 配置源
// @param res *clientv3.GetResponse 初始快照拉取结果
// @return @1 config.Watcher 监听器
// @return @2 error 错误信息
func newWatcher(ctx context.Context, s *Source, res *clientv3.GetResponse) (config.Watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.watcher = clientv3.NewWatcher(w.source.opts.client)
	w.chWatch = make(chan []*config.Configuration, 2)
	w.configs = make(map[string]*config.Configuration)

	if res != nil {
		// 以全量拉取结果作为初始快照
		for _, kv := range res.Kvs {
			c := w.parseKV(kv.Key, kv.Value)
			w.configs[c.FullPath] = c
		}

		// 从拉取时的版本号之后开始监听，避免丢失拉取与监听之间的配置变更
		w.watchCh = w.watcher.Watch(
			w.ctx,
			w.source.opts.path,
			clientv3.WithPrefix(),
			clientv3.WithRev(res.Header.Revision+1),
		)
	}

	w.wg.Go(func() {
		for {
			if w.watchCh == nil {
				// 初始快照拉取失败时，先全量重连并重建监听，避免以不完整的快照启动
				if !w.resync() {
					return
				}
				continue
			}

			w.watchLoop()

			if w.stopped.Load() {
				return
			}

			if !w.resync() {
				return
			}
		}
	})

	return w, nil
}

// Next 返回配置列表
// 阻塞等待配置变更，监听被停止时返回错误
// @return @1 []*config.Configuration 配置项列表
// @return @2 error 错误信息
func (w *watcher) Next() ([]*config.Configuration, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case configs, ok := <-w.chWatch:
		if !ok {
			return nil, errors.ErrWatcherStopped
		}

		return configs, nil
	}
}

// 解析配置
// 将etcd的键值对转换为统一的配置结构
// @param key []byte 配置键名
// @param value []byte 配置内容
// @return @1 *config.Configuration 配置项
func (w *watcher) parseKV(key []byte, value []byte) *config.Configuration {
	fullPath := string(key)
	path := strings.TrimPrefix(fullPath, w.source.opts.path)
	file := filepath.Base(fullPath)
	ext := filepath.Ext(file)

	return &config.Configuration{
		Path:     path,
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  value,
		FullPath: fullPath,
	}
}

// 监听事件循环
// 处理etcd监听事件：PUT更新配置快照，DELETE删除配置快照，然后广播最新配置
func (w *watcher) watchLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case res, ok := <-w.watchCh:
			if !ok {
				return
			}

			if res.Err() != nil {
				log.Warnf("etcd watch error: %v", res.Err())
				return
			}

			w.rw.Lock()
			for _, ev := range res.Events {
				switch ev.Type {
				case mvccpb.PUT:
					c := w.parseKV(ev.Kv.Key, ev.Kv.Value)
					w.configs[c.FullPath] = c
				case mvccpb.DELETE:
					delete(w.configs, string(ev.Kv.Key))
				}
			}
			w.rw.Unlock()

			w.broadcast()
		}
	}
}

// 全量重连并重试
// watch失效后重新拉取全量配置并重建监听，直到成功或监听被停止
// @return @1 bool 是否重建成功
func (w *watcher) resync() bool {
	retryTimes := max(1, w.source.opts.retryTimes)

	for {
		err := xcall.Backoff(w.ctx, func(ctx context.Context, attempt int) (bool, error) {
			if w.stopped.Load() {
				return false, errors.ErrWatcherStopped
			}

			tctx, tcancel := context.WithTimeout(ctx, w.source.opts.timeout)
			res, err := w.source.opts.client.Get(tctx, w.source.opts.path, clientv3.WithPrefix())
			tcancel()
			if err != nil {
				log.Warnf("etcd watch resync failed, retry %d times, err: %v", attempt, err)
				return true, err
			}

			w.rw.Lock()
			w.configs = make(map[string]*config.Configuration)
			for _, kv := range res.Kvs {
				c := w.parseKV(kv.Key, kv.Value)
				w.configs[c.FullPath] = c
			}
			w.rw.Unlock()

			w.broadcast()

			w.watchCh = w.watcher.Watch(
				w.ctx,
				w.source.opts.path,
				clientv3.WithPrefix(),
				clientv3.WithRev(res.Header.Revision+1),
			)

			return false, nil
		}, retryTimes, 100*time.Millisecond, 3*time.Second)
		if err == nil {
			return true
		}

		if w.ctx.Err() != nil || w.stopped.Load() {
			return false
		}
	}
}

// 广播配置列表
// 从配置快照中获取全量配置并通知监听器
func (w *watcher) broadcast() {
	w.rw.RLock()
	configs := w.snapshot()
	w.rw.RUnlock()

	w.notify(configs)
}

// 通知监听器配置列表已更新
// 清空旧数据后非阻塞发送最新配置快照
// @param configs []*config.Configuration 配置项列表
func (w *watcher) notify(configs []*config.Configuration) {
	if w.stopped.Load() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped.Load() {
		return
	}

	w.flush()

	select {
	case w.chWatch <- configs:
	case <-w.ctx.Done():
	}
}

// 清空所有旧数据，仅保留最新配置快照
func (w *watcher) flush() {
	for {
		select {
		case <-w.chWatch:
			// continue
		default:
			return
		}
	}
}

// 返回当前全量配置列表
func (w *watcher) snapshot() []*config.Configuration {
	configs := make([]*config.Configuration, 0, len(w.configs))
	for _, c := range w.configs {
		configs = append(configs, c)
	}

	return configs
}

// Stop 停止监听
// @return @1 error 错误信息
func (w *watcher) Stop() error {
	w.release()

	w.wg.Wait()

	return nil
}

// 释放资源
// 取消上下文、关闭etcd监听器并关闭配置变更通道
func (w *watcher) release() {
	if !w.stopped.CompareAndSwap(false, true) {
		return
	}

	w.cancel()
	w.watcher.Close()

	w.mu.Lock()
	close(w.chWatch)
	w.mu.Unlock()
}
