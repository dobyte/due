package eventbus

import (
	"context"
	"github.com/dobyte/due/internal/value"
	"time"
)

type EventBus interface {
	// Watch 监听事件
	Watch()
	// Stop 停止监听
	Stop() error
	// Publish 发布事件
	Publish(ctx context.Context, topic string, message interface{}) error
	// Subscribe 订阅事件
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	// Unsubscribe 取消订阅
	Unsubscribe(ctx context.Context, topic string, handler EventHandler) error
}

type EventHandler func(event *Event)

type Event struct {
	ID        string      // 事件ID
	Topic     string      // 事件主题
	Payload   value.Value // 事件载荷
	Timestamp time.Time   // 事件时间
}
