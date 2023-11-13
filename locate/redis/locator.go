package redis

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
	"sort"
	"strings"
	"sync"
)

const (
	userGateKey     = "%s:locate:user:%d:gate"     // string
	userNodeKey     = "%s:locate:user:%d:node"     // hash
	clusterEventKey = "%s:locate:cluster:%s:event" // channel
)

var _ locate.Locator = &Locator{}

type Locator struct {
	ctx      context.Context
	cancel   context.CancelFunc
	opts     *options
	sfg      singleflight.Group // singleFlight
	watchers sync.Map
}

func NewLocator(opts ...Option) *Locator {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.prefix == "" {
		o.prefix = defaultPrefix
	}

	if o.client == nil {
		o.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		})
	}

	l := &Locator{}
	l.ctx, l.cancel = context.WithCancel(o.ctx)
	l.opts = o

	return l
}

// LocateGate 定位用户所在网关
func (l *Locator) LocateGate(ctx context.Context, uid int64) (string, error) {
	key := fmt.Sprintf(userGateKey, l.opts.prefix, uid)
	val, err, _ := l.sfg.Do(key, func() (interface{}, error) {
		val, err := l.opts.client.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return "", err
		}

		return val, nil
	})
	if err != nil {
		return "", err
	}

	return val.(string), nil
}

// LocateNode 定位用户所在节点
func (l *Locator) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)
	val, err, _ := l.sfg.Do(key+name, func() (interface{}, error) {
		val, err := l.opts.client.HGet(ctx, key, name).Result()
		if err != nil && err != redis.Nil {
			return "", err
		}

		return val, nil
	})
	if err != nil {
		return "", err
	}

	return val.(string), nil
}

// BindGate 绑定网关
func (l *Locator) BindGate(ctx context.Context, uid int64, gid string) error {
	key := fmt.Sprintf(userGateKey, l.opts.prefix, uid)
	err := l.opts.client.Set(ctx, key, gid, redis.KeepTTL).Err()
	if err != nil {
		return err
	}

	err = l.publish(ctx, locate.BindGate, uid, gid)
	if err != nil {
		log.Errorf("location event publish failed: %v", err)
	}

	return nil
}

// BindNode 绑定节点
func (l *Locator) BindNode(ctx context.Context, uid int64, name, nid string) error {
	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)
	err := l.opts.client.HSet(ctx, key, name, nid).Err()
	if err != nil {
		return err
	}

	err = l.publish(ctx, locate.BindNode, uid, nid, name)
	if err != nil {
		log.Errorf("location event publish failed: %v", err)
	}

	return nil
}

// UnbindGate 解绑网关
func (l *Locator) UnbindGate(ctx context.Context, uid int64, gid string) error {
	oldGID, err := l.LocateGate(ctx, uid)
	if err != nil {
		return err
	}

	if oldGID == "" || oldGID != gid {
		return nil
	}

	key := fmt.Sprintf(userGateKey, l.opts.prefix, uid)
	err = l.opts.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	err = l.publish(ctx, locate.UnbindGate, uid, gid)
	if err != nil {
		log.Errorf("location event publish failed: %v", err)
	}

	return nil
}

// UnbindNode 解绑节点
func (l *Locator) UnbindNode(ctx context.Context, uid int64, name string, nid string) error {
	oldNID, err := l.LocateNode(ctx, uid, name)
	if err != nil {
		return err
	}

	if oldNID == "" || oldNID != nid {
		return nil
	}

	key := fmt.Sprintf(userNodeKey, l.opts.prefix, uid)
	err = l.opts.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	err = l.publish(ctx, locate.UnbindNode, uid, nid, name)
	if err != nil {
		log.Errorf("location event publish failed: %v", err)
	}

	return nil
}

func (l *Locator) publish(ctx context.Context, typ locate.EventType, uid int64, insID string, insName ...string) error {
	var (
		kind string
		name string
	)
	switch typ {
	case locate.BindGate, locate.UnbindGate:
		kind = "gate"
	case locate.BindNode, locate.UnbindNode:
		kind = "node"
	}

	if len(insName) > 0 {
		name = insName[0]
	}

	msg, err := marshal(&locate.Event{
		UID:     uid,
		Type:    typ,
		InsID:   insID,
		InsKind: kind,
		InsName: name,
	})
	if err != nil {
		return err
	}

	return l.opts.client.Publish(ctx, fmt.Sprintf(clusterEventKey, l.opts.prefix, kind), msg).Err()
}

func (l *Locator) toUniqueKey(kinds ...string) string {
	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i] < kinds[j]
	})

	keys := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		keys = append(keys, kind)
	}

	return strings.Join(keys, "&")
}

// Watch 监听用户定位变化
func (l *Locator) Watch(ctx context.Context, kinds ...string) (locate.Watcher, error) {
	key := l.toUniqueKey(kinds...)

	v, ok := l.watchers.Load(key)
	if ok {
		return v.(*watcherMgr).fork(), nil
	}

	w, err := newWatcherMgr(ctx, l, key, kinds...)
	if err != nil {
		return nil, err
	}

	l.watchers.Store(key, w)

	return w.fork(), nil
}

func marshal(event *locate.Event) (string, error) {
	buf, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func unmarshal(data []byte) (*locate.Event, error) {
	event := &locate.Event{}
	if err := json.Unmarshal(data, event); err != nil {
		return nil, err
	}
	return event, nil
}
