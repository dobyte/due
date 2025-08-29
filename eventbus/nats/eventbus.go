package nats

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/eventbus"
	"github.com/nats-io/nats.go"
)

type Eventbus struct {
	err       error
	opts      *options
	builtin   bool
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
		eb.builtin = true
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

	return eb.opts.conn.Publish(eb.doMakeChannel(topic), buf)
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	if eb.err != nil {
		return eb.err
	}

	channel := eb.doMakeChannel(topic)

	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[channel]
	if !ok {
		c = &consumer{handlers: make(map[uintptr][]eventbus.EventHandler)}
		sub, err := eb.opts.conn.Subscribe(channel, func(msg *nats.Msg) {
			c.dispatch(msg.Data)
		})
		if err != nil {
			return err
		}
		c.sub = sub
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

		if err := c.sub.Unsubscribe(); err != nil {
			return err
		}

		delete(eb.consumers, channel)
	}

	return nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	if eb.err != nil {
		return eb.err
	}

	if eb.builtin {
		eb.opts.conn.Close()
	}

	return nil
}

func (eb *Eventbus) doMakeChannel(topic string) string {
	if eb.opts.prefix == "" {
		return topic
	} else {
		return eb.opts.prefix + ":" + topic
	}
}
