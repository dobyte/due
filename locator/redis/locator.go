package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locator"
	"github.com/dobyte/due/log"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
	"sort"
	"strings"
	"sync"
)

const (
	userLocationsKey = "due:locate:user:%d:locations" // hash
	channelEventKey  = "due:locate:channel:%v:event"  // channel
)

var _ locator.Locator = &Locator{}

type Locator struct {
	ctx      context.Context
	cancel   context.CancelFunc
	opts     *options
	sfg      singleflight.Group // singleFlight
	watchers sync.Map
}

func NewLocator(opts ...Option) *Locator {
	o := &options{
		ctx:        context.Background(),
		addrs:      []string{"127.0.0.1:6379"},
		maxRetries: 3,
	}
	for _, opt := range opts {
		opt(o)
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

// Get 获取用户定位
func (l *Locator) Get(ctx context.Context, uid int64, insKind cluster.Kind) (string, error) {
	key := fmt.Sprintf(userLocationsKey, uid)
	val, err, _ := l.sfg.Do(key+insKind.String(), func() (interface{}, error) {
		val, err := l.opts.client.HGet(ctx, key, insKind.String()).Result()
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

// Set 设置用户定位
func (l *Locator) Set(ctx context.Context, uid int64, insKind cluster.Kind, insID string) error {
	key := fmt.Sprintf(userLocationsKey, uid)
	err := l.opts.client.HSet(ctx, key, insKind.String(), insID).Err()
	if err != nil {
		return err
	}

	err = l.publish(ctx, uid, insKind, insID, locator.SetLocation)
	if err != nil {
		log.Errorf("location event publish failed: %v", err)
	}

	return nil
}

// Rem 移除用户定位
func (l *Locator) Rem(ctx context.Context, uid int64, insKind cluster.Kind, insID string) error {
	oldInsID, err := l.Get(ctx, uid, insKind)
	if err != nil {
		return err
	}

	if oldInsID == "" || oldInsID != insID {
		return nil
	}

	key := fmt.Sprintf(userLocationsKey, uid)
	err = l.opts.client.HDel(ctx, key, insKind.String()).Err()
	if err != nil {
		return err
	}

	err = l.publish(ctx, uid, insKind, insID, locator.RemLocation)
	if err != nil {
		log.Errorf("location event publish failed: %v", err)
	}

	return nil
}

func (l *Locator) publish(ctx context.Context, uid int64, insKind cluster.Kind, insID string, eventType locator.EventType) error {
	msg, err := marshal(&locator.Event{
		UID:     uid,
		Type:    eventType,
		InsID:   insID,
		InsKind: insKind,
	})
	if err != nil {
		return err
	}

	channel := fmt.Sprintf(channelEventKey, insKind.String())

	return l.opts.client.Publish(ctx, channel, msg).Err()
}

func (l *Locator) toUniqueKey(insKinds ...cluster.Kind) string {
	sort.Slice(insKinds, func(i, j int) bool {
		return insKinds[i] < insKinds[j]
	})

	keys := make([]string, 0, len(insKinds))
	for _, insKind := range insKinds {
		keys = append(keys, insKind.String())
	}

	return strings.Join(keys, "&")
}

// Watch 监听用户定位变化
func (l *Locator) Watch(ctx context.Context, insKinds ...cluster.Kind) (locator.Watcher, error) {
	key := l.toUniqueKey(insKinds...)

	v, ok := l.watchers.Load(key)
	if ok {
		return v.(*watcherMgr).fork(), nil
	}

	w, err := newWatcherMgr(ctx, l, key, insKinds...)
	if err != nil {
		return nil, err
	}

	l.watchers.Store(key, w)

	return w.fork(), nil
}

func marshal(event *locator.Event) (string, error) {
	buf, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func unmarshal(data []byte) (*locator.Event, error) {
	event := &locator.Event{}
	if err := json.Unmarshal(data, event); err != nil {
		return nil, err
	}
	return event, nil
}
