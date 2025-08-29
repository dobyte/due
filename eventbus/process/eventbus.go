package process

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/core/value"
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
	defer eb.rw.RUnlock()

	c, ok := eb.consumers[topic]
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
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler internal.EventHandler) error {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if !ok {
		c = &consumer{handlers: make(map[uintptr][]internal.EventHandler, 1)}
		eb.consumers[topic] = c
	}

	c.addHandler(handler)

	return nil
}

// Unsubscribe 取消订阅
func (eb *Eventbus) Unsubscribe(ctx context.Context, topic string, handler internal.EventHandler) error {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[topic]; ok {
		if c.delHandler(handler) != 0 {
			return nil
		}

		delete(eb.consumers, topic)
	}

	return nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	return nil
}
