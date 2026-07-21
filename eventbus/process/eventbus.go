package process

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/eventbus/internal"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
)

type Eventbus struct {
	rw        sync.RWMutex
	consumers map[string]*consumer
}

func NewEventbus() *Eventbus {
	eb := &Eventbus{}
	eb.consumers = make(map[string]*consumer)

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload any) error {
	eb.rw.RLock()
	c, ok := eb.consumers[topic]
	eb.rw.RUnlock()
	if !ok {
		return nil
	}

	c.dispatch(&internal.Event{
		ID:        xuuid.UUID(),
		Topic:     topic,
		Payload:   value.NewValue(payload),
		Timestamp: xtime.UnixNano(xtime.Now().UnixNano()),
	})

	return nil
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, opts ...eventbus.SubscribeOptions) (eventbus.Subscription, error) {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if ok {
		if len(opts) > 0 && opts[0].IsSingleConsumer != c.isSingleConsumer {
			return nil, errors.ErrInvalidArgument
		}
	} else {
		c = &consumer{isSingleConsumer: len(opts) > 0 && opts[0].IsSingleConsumer, subscriptions: make([]*subscription, 0, 1)}
		eb.consumers[topic] = c
	}

	sub := c.addSubscription(topic, handler)
	sub.eb = eb

	return sub, nil
}

// 取消订阅
func (eb *Eventbus) unsubscribe(topic string, sub *subscription) error {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if !ok {
		return errors.ErrIllegalOperation
	}

	found, empty := c.delSubscription(sub)
	if !found {
		return errors.ErrIllegalOperation
	}

	if empty {
		delete(eb.consumers, topic)
	}

	return nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	return nil
}
