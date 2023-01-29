package redis

import (
	"context"
	"encoding/json"
	"github.com/dobyte/due/eventbus"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/task"
	"github.com/dobyte/due/utils/xuuid"
	"github.com/go-redis/redis/v8"
	"sync"
)

type EventBus struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options
	sub    *redis.PubSub

	rw       sync.RWMutex
	handlers map[string]map[*eventbus.Handler]struct{}
}

func NewEventBus(opts ...Option) *EventBus {
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

	eb := &EventBus{}
	eb.ctx, eb.cancel = context.WithCancel(o.ctx)
	eb.opts = o
	eb.sub = eb.opts.client.Subscribe(eb.ctx)
	eb.handlers = make(map[string]map[*eventbus.Handler]struct{})

	return eb
}

// Publish 发布事件
func (eb *EventBus) Publish(ctx context.Context, topic string, message interface{}) error {
	id, err := xuuid.UUID()
	if err != nil {
		return err
	}

	channel := eb.opts.prefix + ":" + topic
	payload := &eventbus.Payload{
		ID:      id,
		Topic:   topic,
		Message: message,
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return eb.opts.client.Publish(ctx, channel, buf).Err()
}

// Subscribe 订阅事件
func (eb *EventBus) Subscribe(ctx context.Context, topic string, handler eventbus.Handler) error {
	eb.rw.Lock()
	if _, ok := eb.handlers[topic]; !ok {
		eb.handlers[topic] = make(map[*eventbus.Handler]struct{}, 1)
	}
	eb.handlers[topic][&handler] = struct{}{}
	eb.rw.Unlock()

	return eb.sub.Subscribe(ctx, eb.opts.prefix+":"+topic)
}

// Unsubscribe 取消订阅
func (eb *EventBus) Unsubscribe(ctx context.Context, topic string, handler eventbus.Handler) error {
	eb.rw.Lock()
	if handlers, ok := eb.handlers[topic]; ok {
		if _, ok = handlers[&handler]; ok {
			delete(handlers, &handler)
		}

		if len(handlers) == 0 {
			delete(eb.handlers, topic)
		}
	}
	eb.rw.Unlock()

	return eb.sub.Unsubscribe(ctx, eb.opts.prefix+":"+topic)
}

// Watch 监听事件
func (eb *EventBus) Watch() {
	for {
		iface, err := eb.sub.Receive(eb.ctx)
		if err != nil {
			return
		}

		switch v := iface.(type) {
		case *redis.Subscription:
			log.Debugf("channel subscribe succeeded, %s", v.Channel)
		case *redis.Message:
			payload := &eventbus.Payload{}
			if err := json.Unmarshal([]byte(v.Payload), payload); err != nil {
				log.Errorf("invalid payload, %s", v.Payload)
				continue
			}

			func(payload *eventbus.Payload) {
				eb.rw.RLock()
				defer eb.rw.RUnlock()

				handlers, ok := eb.handlers[payload.Topic]
				if !ok {
					return
				}

				for handler := range handlers {
					fn := *handler
					if err = task.AddTask(func() { fn(payload) }); err != nil {
						log.Warnf("task add failed, system auto switch to blocking invoke, err: %v", err)
						fn(payload)
					}
				}
			}(payload)
		}
	}
}

// Stop 停止监听
func (eb *EventBus) Stop() error {
	eb.cancel()
	return eb.sub.Close()
}
