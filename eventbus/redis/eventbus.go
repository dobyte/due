package redis

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/redis/go-redis/v9"
)

type Eventbus struct {
	err       error
	ctx       context.Context
	cancel    context.CancelFunc
	builtin   bool
	opts      *options
	sub       *redis.PubSub
	rw        sync.RWMutex
	consumers map[string]*consumer
}

func NewEventbus(opts ...Option) *Eventbus {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	eb := &Eventbus{}

	defer func() {
		if eb.err == nil {
			eb.opts = o
			eb.ctx, eb.cancel = context.WithCancel(o.ctx)
			eb.sub = eb.opts.client.Subscribe(eb.ctx)
			eb.consumers = make(map[string]*consumer)

			go eb.watch()
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
			if options.TLSConfig, eb.err = tls.MakeRedisTLSConfig(o.certFile, o.keyFile, o.caFile); eb.err != nil {
				return eb
			}
		}

		o.client, eb.builtin = redis.NewUniversalClient(options), true
	}

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload any) error {
	if eb.err != nil {
		return eb.err
	}

	buf, err := serialize(topic, payload)
	if err != nil {
		return err
	}

	return eb.opts.client.Publish(ctx, eb.doMakeChannel(topic), buf).Err()
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	if eb.err != nil {
		return eb.err
	}

	channel := eb.doMakeChannel(topic)

	if err := eb.sub.Subscribe(ctx, channel); err != nil {
		return err
	}

	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[channel]
	if !ok {
		c = &consumer{handlers: make(map[uintptr][]eventbus.EventHandler, 1)}
		eb.consumers[channel] = c
	}

	c.addHandler(handler)

	return nil
}

// Unsubscribe 取消订阅
func (eb *Eventbus) Unsubscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	if eb.err != nil {
		return eb.err
	}

	channel := eb.doMakeChannel(topic)

	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[channel]; ok {
		if c.delHandler(handler) != 0 {
			return nil
		}

		if err := eb.sub.Unsubscribe(ctx, channel); err != nil {
			return err
		}

		delete(eb.consumers, channel)
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
		case *redis.Message:
			eb.rw.RLock()
			c, ok := eb.consumers[v.Channel]
			eb.rw.RUnlock()
			if ok {
				c.dispatch(xconv.Bytes(v.Payload))
			}
		}
	}
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	if eb.err != nil {
		return eb.err
	}

	eb.cancel()

	if eb.builtin {
		_ = eb.sub.Close()

		return eb.opts.client.Close()
	} else {
		return eb.sub.Close()
	}
}

func (eb *Eventbus) doMakeChannel(topic string) string {
	if eb.opts.prefix == "" {
		return topic
	} else {
		return eb.opts.prefix + ":" + topic
	}
}
