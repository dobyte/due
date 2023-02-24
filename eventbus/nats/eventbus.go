package nats

import (
	"context"
	"github.com/dobyte/due/eventbus"
	"github.com/nats-io/nats.go"
	"sync"
)

type Eventbus struct {
	err  error
	opts *options

	rw        sync.RWMutex
	consumers map[string]*consumer
}

func NewEventbus(opts ...Option) *Eventbus {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	eb := &Eventbus{opts: o}
	eb.opts = o
	eb.consumers = make(map[string]*consumer)

	if o.conn == nil {
		o.conn, eb.err = nats.Connect(o.url, nats.Timeout(o.timeout))
	}

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload interface{}) error {
	if eb.err != nil {
		return eb.err
	}

	buf, err := eventbus.PackData(topic, payload)
	if err != nil {
		return err
	}

	return eb.opts.conn.Publish(topic, buf)
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	if eb.err != nil {
		return eb.err
	}

	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if !ok {
		c = &consumer{handlers: make(map[uintptr]eventbus.EventHandler)}
		sub, err := eb.opts.conn.Subscribe(topic, func(msg *nats.Msg) {
			c.dispatch(msg.Data)
		})
		if err != nil {
			return err
		}
		c.sub = sub
	}

	c.addHandler(handler)
	eb.consumers[topic] = c

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

		if err := c.sub.Unsubscribe(); err != nil {
			return err
		}

		delete(eb.consumers, topic)
	}

	return nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	eb.opts.conn.Close()
	return nil
}
