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
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, balance ...bool) (eventbus.Subscription, error) {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if ok {
		if len(balance) > 0 && balance[0] != c.balance {
			return nil, errors.ErrInvalidArgument
		}
	} else {
		c = &consumer{balance: len(balance) > 0 && balance[0], subscriptions: make([]*subscription, 0, 1)}
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
