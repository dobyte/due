package eventbus

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
)

var globalEventbus Eventbus

func init() {
	SetEventbus(NewEventbus())
}

type EventHandler func(event *Event)

type Event struct {
	ID        string      // 事件ID
	Topic     string      // 事件主题
	Payload   value.Value // 事件载荷
	Timestamp time.Time   // 事件时间
}

type Eventbus interface {
	// Close 关闭事件总线
	Close() error
	// Publish 发布事件
	Publish(ctx context.Context, topic string, message any) error
	// Subscribe 订阅事件
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	// Unsubscribe 取消订阅
	Unsubscribe(ctx context.Context, topic string, handler EventHandler) error
}

type defaultEventbus struct {
	ctx    context.Context
	cancel context.CancelFunc

	rw        sync.RWMutex
	consumers map[string]*consumer
}

func NewEventbus() *defaultEventbus {
	eb := &defaultEventbus{}
	eb.consumers = make(map[string]*consumer)

	return eb
}

// Publish 发布事件
func (eb *defaultEventbus) Publish(ctx context.Context, topic string, payload any) error {
	eb.rw.RLock()
	defer eb.rw.RUnlock()

	c, ok := eb.consumers[topic]
	if !ok {
		return nil
	}

	c.dispatch(&Event{
		ID:        xuuid.UUID(),
		Topic:     topic,
		Payload:   value.NewValue(payload),
		Timestamp: xtime.UnixNano(xtime.Now().UnixNano()),
	})

	return nil
}

// Subscribe 订阅事件
func (eb *defaultEventbus) Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if !ok {
		c = &consumer{handlers: make(map[uintptr][]EventHandler)}
		eb.consumers[topic] = c
	}

	c.addHandler(handler)

	return nil
}

// Unsubscribe 取消订阅
func (eb *defaultEventbus) Unsubscribe(ctx context.Context, topic string, handler EventHandler) error {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[topic]; ok {
		if c.remHandler(handler) != 0 {
			return nil
		}

		delete(eb.consumers, topic)
	}

	return nil
}

// Close 停止监听
func (eb *defaultEventbus) Close() error {
	return nil
}

// SetEventbus 设置事件总线
func SetEventbus(eb Eventbus) {
	if eb == nil {
		log.Warn("cannot set a nil eventbus")
		return
	}

	if globalEventbus != nil {
		if err := globalEventbus.Close(); err != nil {
			log.Errorf("the old eventbus close failed: %v", err)
		}
	}

	globalEventbus = eb
}

// GetEventbus 获取事件总线
func GetEventbus() Eventbus {
	return globalEventbus
}

// Publish 发布事件
func Publish(ctx context.Context, topic string, message any) error {
	return globalEventbus.Publish(ctx, topic, message)
}

// Subscribe 订阅事件
func Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	return globalEventbus.Subscribe(ctx, topic, handler)
}

// Unsubscribe 取消订阅
func Unsubscribe(ctx context.Context, topic string, handler EventHandler) error {
	return globalEventbus.Unsubscribe(ctx, topic, handler)
}

// Close 关闭事件总线
func Close() error {
	return globalEventbus.Close()
}
