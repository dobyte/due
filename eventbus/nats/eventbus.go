package nats

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/nats-io/nats.go"
)

type Eventbus struct {
	err     error
	opts    *options
	builtin bool
	pool    *sync.Pool
}

func NewEventbus(opts ...Option) *Eventbus {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	eb := &Eventbus{}
	eb.opts = o
	eb.pool = &sync.Pool{New: func() any { return &data{} }}

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

	buf, err := eb.serialize(topic, payload)
	if err != nil {
		return err
	}

	return eb.opts.conn.Publish(eb.doMakeChannel(topic), buf)
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, opts ...eventbus.SubscribeOptions) (eventbus.Subscription, error) {
	if eb.err != nil {
		return nil, eb.err
	}

	var (
		err     error
		sub     *nats.Subscription
		channel = eb.doMakeChannel(topic)
	)

	if len(opts) > 0 && opts[0].IsSingleConsumer {
		sub, err = eb.opts.conn.QueueSubscribe(channel, "queue", eb.subscribeHandler(handler))
	} else {
		sub, err = eb.opts.conn.Subscribe(channel, eb.subscribeHandler(handler))
	}
	if err != nil {
		return nil, err
	}

	return &subscription{sub: sub}, nil
}

// 订阅事件处理函数
func (eb *Eventbus) subscribeHandler(handler eventbus.EventHandler) func(msg *nats.Msg) {
	return func(msg *nats.Msg) {
		event, err := eb.deserialize(msg.Data)
		if err != nil {
			log.Errorf("invalid event data: %v", err)
			return
		}

		handler(event)
	}
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

// 序列化
func (eb *Eventbus) serialize(topic string, payload any) ([]byte, error) {
	d := eb.pool.Get().(*data)
	defer eb.pool.Put(d)

	d.ID = xuuid.UUID()
	d.Topic = topic
	d.Payload = xconv.String(payload)
	d.Timestamp = xtime.Now().UnixNano()

	return json.Marshal(d)
}

// 反序列化
func (eb *Eventbus) deserialize(v []byte) (*eventbus.Event, error) {
	d := eb.pool.Get().(*data)
	defer eb.pool.Put(d)

	if err := json.Unmarshal(v, d); err != nil {
		return nil, err
	}

	return &eventbus.Event{
		ID:        d.ID,
		Topic:     d.Topic,
		Payload:   value.NewValue(d.Payload),
		Timestamp: xtime.UnixNano(d.Timestamp),
	}, nil
}
