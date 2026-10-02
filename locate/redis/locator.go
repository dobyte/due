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

// Locator is a redis locator.
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

// NewLocator creates a redis locator.
//
// It initializes the locator and a built-in redis client; when no external client is injected it
// connects to 127.0.0.1:6379 by default.
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

// Name returns the component name of the locator.
func (l *Locator) Name() string {
	return name
}

// LocateGate locates the gate the user belongs to.
//
// It returns the gate ID of the user and an error when the lookup fails.
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

// LocateNode locates the node the user belongs to.
//
// It returns the node ID of the user and an error when the lookup fails.
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

// LocateNodes locates the nodes the user belongs to.
//
// It returns a mapping from the node names the user is bound to, to their node IDs, and an error
// when the lookup fails.
func (l *Locator) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if l.err != nil {
		return nil, l.err
	}

	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)

	return l.opts.client.HGetAll(ctx, key).Result()
}

// BindGate binds the user to a gate.
//
// It returns an error when binding fails.
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

// BindNode binds the user to a node.
//
// It returns an error when binding fails.
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

// UnbindGate unbinds the user from a gate.
//
// It returns an error when unbinding fails.
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

// UnbindNode unbinds the user from a node.
//
// It returns an error when unbinding fails.
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

// Close closes the locator.
//
// It returns an error when closing fails.
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

// broadcast broadcasts a locate event.
//
// It broadcasts a locate event to every listener through redis publish/subscribe and returns an
// error when broadcasting fails.
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

// Watch watches changes of user locations.
//
// It returns a locate watcher and an error when watching fails.
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

// doBuildWatcherMgr builds a locate watch manager.
//
// It reuses the watch manager of the same combination of instance kinds and creates a new one when
// none exists.
//
// It returns the locate watch manager and an error when building fails.
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

	// Handle the race where the receiving goroutine has already stopped because the reconnect
	// failed completely before Store.
	if mgr.stopped.Load() {
		l.watchers.Delete(key)
		return nil, errors.ErrWatcherStopped
	}

	return mgr, nil
}
