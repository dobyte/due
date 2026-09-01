package consul

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

// 监听器
type watcher struct {
	ctx     context.Context              // 上下文
	cancel  context.CancelFunc           // 取消函数
	source  *Source                      // 配置源
	plan    *watch.Plan                  // 监听计划
	chWatch chan []*config.Configuration // 配置变更通道
}

// 创建监听器
// @param ctx context.Context 上下文
// @param s *Source 配置源
// @return @1 config.Watcher 监听器
// @return @2 error 错误信息
func newWatcher(ctx context.Context, s *Source) (config.Watcher, error) {
	w := &watcher{}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.source = s
	w.chWatch = make(chan []*config.Configuration, 2)

	if err := w.init(); err != nil {
		return nil, err
	}

	return w, nil
}

// 初始化监听器
// 解析keyprefix监听计划并启动监听协程
// @return @1 error 错误信息
func (w *watcher) init() (err error) {
	var prefix string
	if w.source.opts.path != "" {
		prefix = w.source.opts.path + "/"
	}

	w.plan, err = watch.Parse(map[string]any{
		"type":   "keyprefix",
		"prefix": prefix,
	})
	if err != nil {
		return
	}

	w.plan.Handler = w.planHandler

	xcall.Go(func() {
		for {
			if runErr := w.plan.RunWithClientAndHclog(w.source.opts.client, nil); runErr != nil {
				if w.ctx.Err() != nil {
					return
				}

				log.Warnf("consul watch failed: %v", runErr)

				select {
				case <-w.ctx.Done():
					return
				case <-time.After(time.Second):
				}

				continue
			}

			return
		}
	})

	return
}

// 处理监听计划回调
// 将Consul返回的KV集合转换为配置项列表并通知监听器
// @param idx uint64 索引值
// @param raw any 原始回调数据
func (w *watcher) planHandler(idx uint64, raw any) {
	if raw == nil {
		return // ignore
	}

	kvs, ok := raw.(api.KVPairs)
	if !ok {
		return
	}

	configs := make([]*config.Configuration, 0, len(kvs))
	for _, kv := range kvs {
		configs = append(configs, w.parseKV(kv.Key, kv.Value))
	}

	w.notify(configs)
}

// 解析KV
// 将Consul的KV转换为统一的配置结构
// @param key string 配置键名
// @param value []byte 配置内容
// @return @1 *config.Configuration 配置项
func (w *watcher) parseKV(key string, value []byte) *config.Configuration {
	fullPath := key
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

// 通知监听器配置列表已更新
// 清空旧数据后非阻塞发送最新配置快照
// @param configs []*config.Configuration 配置项列表
func (w *watcher) notify(configs []*config.Configuration) {
	if w.ctx.Err() != nil {
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
		default:
			return
		}
	}
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

// Stop 停止监听
// @return @1 error 错误信息
func (w *watcher) Stop() error {
	w.cancel()
	w.plan.Stop()

	return nil
}
