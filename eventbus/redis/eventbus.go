package redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/sasha-s/go-deadlock"
	"github.com/symsimmy/due/eventbus"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/task"
	"github.com/symsimmy/due/utils/xconv"
	"reflect"
)

type Eventbus struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options
	sub    *redis.PubSub

	rw       deadlock.RWMutex
	handlers map[string]map[uintptr]eventbus.EventHandler
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
	eb.handlers = make(map[string]map[uintptr]eventbus.EventHandler)
	go eb.watch()

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload interface{}) error {
	buf, err := eventbus.PackData(topic, payload)
	if err != nil {
		return err
	}

	return eb.opts.client.Publish(ctx, eb.opts.prefix+":"+topic, buf).Err()
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	err := eb.sub.Subscribe(ctx, eb.opts.prefix+":"+topic)
	if err != nil {
		return err
	}

	pointer := reflect.ValueOf(handler).Pointer()

	eb.rw.Lock()
	defer eb.rw.Unlock()

	if _, ok := eb.handlers[topic]; !ok {
		eb.handlers[topic] = make(map[uintptr]eventbus.EventHandler, 1)
	}

	eb.handlers[topic][pointer] = handler

	return nil
}

// Unsubscribe 取消订阅
func (eb *Eventbus) Unsubscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	isUnsubscribe := false
	pointer := reflect.ValueOf(handler).Pointer()

	eb.rw.Lock()
	defer eb.rw.Unlock()

	if _, ok := eb.handlers[topic]; ok {
		if _, ok = eb.handlers[topic][pointer]; ok {
			delete(eb.handlers[topic], pointer)
		}

		if len(eb.handlers[topic]) == 0 {
			isUnsubscribe = true
			delete(eb.handlers, topic)
		}
	}

	if isUnsubscribe {
		err := eb.sub.Unsubscribe(ctx, eb.opts.prefix+":"+topic)
		if err != nil {
			return err
		}
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
			eb.dispatch(xconv.Bytes(v.Payload))
		}
	}
}

// 分发事件
func (eb *Eventbus) dispatch(data []byte) {
	event, err := eventbus.UnpackData(data)
	if err != nil {
		log.Errorf("invalid event data")
		return
	}

	eb.rw.RLock()
	defer eb.rw.RUnlock()

	handlers, ok := eb.handlers[event.Topic]
	if !ok {
		return
	}

	for _, handler := range handlers {
		fn := handler
		task.AddTask(func() { fn(event) })
	}
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	eb.cancel()
	return eb.sub.Close()
}
