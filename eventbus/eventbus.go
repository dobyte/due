package eventbus

import (
	"context"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/eventbus/internal"
	"github.com/dobyte/due/v2/log"
)

var globalEventbus Eventbus

type (
	Event        = internal.Event
	EventHandler = internal.EventHandler
)

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
	if globalEventbus == nil {
		return errors.ErrMissingEventbusInstance
	}

	return globalEventbus.Publish(ctx, topic, message)
}

// Subscribe 订阅事件
func Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	if globalEventbus == nil {
		return errors.ErrMissingEventbusInstance
	}

	return globalEventbus.Subscribe(ctx, topic, handler)
}

// Unsubscribe 取消订阅
func Unsubscribe(ctx context.Context, topic string, handler EventHandler) error {
	if globalEventbus == nil {
		return errors.ErrMissingEventbusInstance
	}

	return globalEventbus.Unsubscribe(ctx, topic, handler)
}

// Close 关闭事件总线
func Close() error {
	if globalEventbus == nil {
		return nil
	}

	return globalEventbus.Close()
}
