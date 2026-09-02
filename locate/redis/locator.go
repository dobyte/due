package redis

import (
	"context"
	"fmt"
	"sync"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/redis/go-redis/v9"
)

const (
	userGateKey     = "%s:locate:user:%d:gate"           // string
	userNodeKey     = "%s:locate:user:%d:node"           // hash
	clusterEventKey = "%s:locate:db:%d:cluster:%s:event" // channel
)

const name = "redis"

var _ locate.Locator = &Locator{}

// Locator Redis定位器
type Locator struct {
	err              error
	opts             *options
	builtin          bool
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.Mutex
	watchers         sync.Map
	unbindGateScript *redis.Script
	unbindNodeScript *redis.Script
}

// NewLocator 创建Redis定位器
// 初始化定位器及内建Redis客户端，未注入外部客户端时默认连接127.0.0.1:6379
// @param opts ...Option 定位器配置项
// @return @1 *Locator Redis定位器实例
func NewLocator(opts ...Option) *Locator {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	l := &Locator{}

	defer func() {
		if l.err == nil {
			l.opts = o
			l.ctx, l.cancel = context.WithCancel(o.ctx)
			l.unbindGateScript = redis.NewScript(unbindGateScript)
			l.unbindNodeScript = redis.NewScript(unbindNodeScript)
		}
	}()

	if o.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		}

		if o.certFile != "" && o.keyFile != "" && o.caFile != "" {
			if options.TLSConfig, l.err = tls.MakeRedisTLSConfig(o.certFile, o.keyFile, o.caFile); l.err != nil {
				return l
			}
		}

		o.client, l.builtin = redis.NewUniversalClient(options), true
	}

	return l
}

// Name 获取定位器组件名
// @return @1 string 定位器组件名
func (l *Locator) Name() string {
	return name
}

// LocateGate 定位用户所在网关
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @return @1 string 用户所在的网关ID
// @return @2 error 定位失败时返回的错误
func (l *Locator) LocateGate(ctx context.Context, uid int64) (string, error) {
	if l.err != nil {
		return "", l.err
	}

	key := fmt.Sprintf(userGateKey, l.opts.prefix, uid)

	if val, err := l.opts.client.Get(ctx, key).Result(); err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	} else {
		return val, nil
	}
}

// LocateNode 定位用户所在节点
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @return @1 string 用户所在的节点ID
// @return @2 error 定位失败时返回的错误
func (l *Locator) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if l.err != nil {
		return "", l.err
	}

	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)

	if val, err := l.opts.client.HGet(ctx, key, name).Result(); err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	} else {
		return val, nil
	}
}

// LocateNodes 定位用户所在节点列表
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @return @1 map[string]string 用户绑定的节点名称到节点ID的映射
// @return @2 error 定位失败时返回的错误
func (l *Locator) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if l.err != nil {
		return nil, l.err
	}

	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)

	return l.opts.client.HGetAll(ctx, key).Result()
}

// BindGate 绑定网关
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param gid string 网关ID
// @return @1 error 绑定失败时返回的错误
func (l *Locator) BindGate(ctx context.Context, uid int64, gid string) error {
	if l.err != nil {
		return l.err
	}

	key := fmt.Sprintf(userGateKey, l.opts.prefix, uid)

	if err := l.opts.client.Set(ctx, key, gid, redis.KeepTTL).Err(); err != nil {
		return err
	}

	if err := l.broadcast(ctx, locate.BindGate, uid, gid); err != nil {
		log.Errorf("location event broadcast failed: %v", err)
	}

	return nil
}

// BindNode 绑定节点
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @param nid string 节点ID
// @return @1 error 绑定失败时返回的错误
func (l *Locator) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.err != nil {
		return l.err
	}

	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)

	if err := l.opts.client.HSet(ctx, key, name, nid).Err(); err != nil {
		return err
	}

	if err := l.broadcast(ctx, locate.BindNode, uid, nid, name); err != nil {
		log.Errorf("location event broadcast failed: %v", err)
	}

	return nil
}

// UnbindGate 解绑网关
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param gid string 网关ID
// @return @1 error 解绑失败时返回的错误
func (l *Locator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	if l.err != nil {
		return l.err
	}

	key := fmt.Sprintf(userGateKey, l.opts.prefix, uid)

	rst, err := l.unbindGateScript.Run(ctx, l.opts.client, []string{key}, gid).StringSlice()
	if err != nil {
		return err
	}

	if len(rst) > 0 && rst[0] == "OK" {
		if err = l.broadcast(ctx, locate.UnbindGate, uid, gid); err != nil {
			log.Errorf("location event broadcast failed: %v", err)
		}
	}

	return nil
}

// UnbindNode 解绑节点
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @param nid string 节点ID
// @return @1 error 解绑失败时返回的错误
func (l *Locator) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.err != nil {
		return l.err
	}

	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)

	rst, err := l.unbindNodeScript.Run(ctx, l.opts.client, []string{key}, name, nid).StringSlice()
	if err != nil {
		return err
	}

	if len(rst) > 0 && rst[0] == "OK" {
		if err = l.broadcast(ctx, locate.UnbindNode, uid, nid, name); err != nil {
			log.Errorf("location event broadcast failed: %v", err)
		}
	}

	return nil
}

// Close 关闭定位器
// @return @1 error 关闭失败时返回的错误
func (l *Locator) Close() error {
	if l.err != nil {
		return l.err
	}

	l.cancel()

	l.watchers.Range(func(key, value any) bool {
		if wm, ok := value.(*watcherMgr); ok {
			wm.cancel()
			wm.wg.Wait()
		}
		l.watchers.Delete(key)
		return true
	})

	if l.builtin {
		return l.opts.client.Close()
	}

	return nil
}

// 广播定位事件
// 将定位事件通过Redis发布订阅广播给所有监听者
// @param ctx context.Context 上下文
// @param typ locate.EventType 事件类型
// @param uid int64 用户ID
// @param insID string 实例ID
// @param insName ...string 可选，实例名称
// @return @1 error 广播失败时返回的错误
func (l *Locator) broadcast(ctx context.Context, typ locate.EventType, uid int64, insID string, insName ...string) error {
	evt := &locate.Event{UID: uid, Type: typ, InsID: insID}

	switch typ {
	case locate.BindGate, locate.UnbindGate:
		evt.InsKind = cluster.Gate.String()
	case locate.BindNode, locate.UnbindNode:
		evt.InsKind = cluster.Node.String()
	}

	if len(insName) > 0 {
		evt.InsName = insName[0]
	}

	msg, err := marshal(evt)
	if err != nil {
		return err
	}

	return l.opts.client.Publish(ctx, fmt.Sprintf(clusterEventKey, l.opts.prefix, l.opts.db, evt.InsKind), msg).Err()
}

// Watch 监听用户定位变化
// @param ctx context.Context 上下文
// @param kinds ...string 实例类型列表
// @return @1 locate.Watcher 定位监听器
// @return @2 error 监听失败时返回的错误
func (l *Locator) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	if l.err != nil {
		return nil, l.err
	}

	mgr, err := l.doBuildWatcherMgr(kinds...)
	if err != nil {
		return nil, err
	}

	return mgr.fork()
}

// 构建定位管理器
// 复用相同实例类型组合的监听管理器，不存在时创建新的监听管理器
// @param kinds ...string 实例类型列表
// @return @1 *watcherMgr 定位监听管理器
// @return @2 error 构建失败时返回的错误
func (l *Locator) doBuildWatcherMgr(kinds ...string) (*watcherMgr, error) {
	key := toUniqueKey(kinds...)

	v, ok := l.watchers.Load(key)
	if ok {
		return v.(*watcherMgr), nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if v, ok = l.watchers.Load(key); ok {
		return v.(*watcherMgr), nil
	}

	mgr, err := newWatcherMgr(l, key, kinds...)
	if err != nil {
		return nil, err
	}

	l.watchers.Store(key, mgr)

	// 处理接收协程在 Store 前已因重连彻底失败而停止的竞态
	if mgr.stopped.Load() {
		l.watchers.Delete(key)
		return nil, errors.ErrWatcherStopped
	}

	return mgr, nil
}
