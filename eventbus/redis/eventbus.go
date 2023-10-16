package redis

import (
	"context"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/go-redis/redis/v8"
	"strings"
	"sync"
)

type Eventbus struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options
	sub    *redis.PubSub

	rw        sync.RWMutex
	consumers map[string]*consumer
}

func NewEventbus(opts ...Option) *Eventbus {
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

	eb := &Eventbus{}
	eb.ctx, eb.cancel = context.WithCancel(o.ctx)
	eb.opts = o
	eb.sub = eb.opts.client.Subscribe(eb.ctx)
	eb.consumers = make(map[string]*consumer)
	go eb.watch()

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload interface{}) error {
	buf, err := serialize(topic, payload)
	if err != nil {
		return err
	}

	return eb.opts.client.Publish(ctx, eb.buildChannelKey(topic), buf).Err()
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	err := eb.sub.Subscribe(ctx, eb.opts.prefix+":"+topic)
	if err != nil {
		return err
	}

	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if !ok {
		c = &consumer{handlers: make(map[uintptr]eventbus.EventHandler, 1)}
		eb.consumers[topic] = c
	}

	c.addHandler(handler)

	return nil
}

// Unsubscribe 取消订阅
func (eb *Eventbus) Unsubscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[topic]; ok {
		if c.remHandler(handler) != 0 {
			return nil
		}

		err := eb.sub.Unsubscribe(ctx, eb.buildChannelKey(topic))
		if err != nil {
			return err
		}

		delete(eb.consumers, topic)
	}

	return nil
}

// watch 监听事件
func (eb *Eventbus) watch() {
	for {
		iface, err := eb.sub.Receive(eb.ctx)
		if err != nil {
			return
		}

		switch v := iface.(type) {
		case *redis.Subscription:
			log.Debugf("channel subscribe succeeded, %s", v.Channel)
		case *redis.Message:
			topic := eb.parseChannelKey(v.Channel)

			eb.rw.RLock()
			c, ok := eb.consumers[topic]
			eb.rw.RUnlock()
			if ok {
				c.dispatch(xconv.Bytes(v.Payload))
			}
		}
	}
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	eb.cancel()
	return eb.sub.Close()
}

// build channel key pass by topic
func (eb *Eventbus) buildChannelKey(topic string) string {
	if eb.opts.prefix == "" {
		return topic
	} else {
		return eb.opts.prefix + ":" + topic
	}
}

// parse to topic from channel key
func (eb *Eventbus) parseChannelKey(channel string) string {
	if eb.opts.prefix == "" {
		return channel
	} else {
		return strings.TrimPrefix(channel, eb.opts.prefix+":")
	}
}
