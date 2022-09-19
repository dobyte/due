package redis

import (
	"context"
	"fmt"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locate"
	"github.com/dobyte/due/log"
	"github.com/go-redis/redis/v8"
	"strconv"
	"strings"
)

const (
	userLocationsKey = "due:locate:user:%d:locations" // hash
	channelEventKey  = "due:locate:channel:%v:event"  // channel
)

const (
	setScript = `
	redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
	redis.call('PUBLISH', KEYS[2], ARGV[3])
`
)

var _ locate.Locator = &Locator{}

type Locator struct {
	opts *options
}

func NewLocator(opts ...Option) *Locator {
	o := &options{
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

	return &Locator{opts: o}
}

// Get 获取用户定位
func (l *Locator) Get(ctx context.Context, uid int64, insKind cluster.Kind) (string, error) {
	key := fmt.Sprintf(userLocationsKey, uid)
	return l.opts.client.HGet(ctx, key, insKind.String()).Result()
}

// Set 设置用户定位
func (l *Locator) Set(ctx context.Context, uid int64, insKind cluster.Kind, insID string) error {
	key := fmt.Sprintf(userLocationsKey, uid)
	err := l.opts.client.HSet(ctx, key, insKind.String(), insID).Err()
	if err != nil {
		return err
	}

	l.publish(ctx, uid, insKind, insID, locate.SetLocation)

	return nil
}

// Rem 移除用户定位
func (l *Locator) Rem(ctx context.Context, uid int64, insKind cluster.Kind) error {
	key := fmt.Sprintf(userLocationsKey, uid)
	err := l.opts.client.HDel(ctx, key, insKind.String()).Err()
	if err != nil {
		return err
	}

	l.publish(ctx, uid, insKind, "", locate.SetLocation)

	return nil
}

func (l *Locator) publish(ctx context.Context, uid int64, insKind cluster.Kind, insID string, eventType locate.EventType) {
	msg := fmt.Sprintf("%d@%d@%s@%d", uid, insKind, insID, locate.SetLocation)
	channel := fmt.Sprintf(channelEventKey, insKind)
	err := l.opts.client.Publish(ctx, channel, msg).Err()
	if err != nil {
		log.Errorf("the user location event publish failed: %v", err)
	}
}

// Watch 监听用户定位变化
func (l *Locator) Watch(ctx context.Context, insKinds ...cluster.Kind) (locate.Watcher, error) {
	channels := make([]string, 0, len(insKinds))
	for _, insKind := range insKinds {
		channels = append(channels, fmt.Sprintf(channelEventKey, insKind.String()))
	}

	sub := l.opts.client.Subscribe(ctx, channels...)

	for {
		iface, err := sub.Receive(ctx)
		if err != nil {
			return nil, nil
		}

		switch v := iface.(type) {
		case *redis.Subscription:
			fmt.Println(111)
			log.Debugf("channel subscribe succeeded, %s", v.Channel)
		case *redis.Message:
			fmt.Println(222)
			slice := strings.Split(v.Payload, "@")
			if len(slice) != 4 {
				log.Errorf("invalid synchronize payload, %s", v.Payload)
				continue
			}

			uid, err := strconv.ParseInt(slice[0], 10, 64)
			if err != nil {
				log.Errorf("invalid synchronize payload, %s", v.Payload)
				continue
			}

			fmt.Println(uid)

			//switch slice[3] {
			//case enum.BindAction:
			//	p.sourceGate.Store(uid, slice[2])
			//case enum.UnbindAction:
			//	p.sourceGate.Delete(uid)
			//}
		default:
			fmt.Println(333)
			// handle error
		}
	}

	return nil, nil
}
