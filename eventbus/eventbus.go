package eventbus

import "context"

type EventBus interface {
	// Watch 监听事件
	Watch()
	// Stop 停止监听
	Stop() error
	// Publish 发布事件
	Publish(ctx context.Context, topic string, payload interface{}) error
	// Subscribe 订阅事件
	Subscribe(ctx context.Context, topic string, handler Handler) error
	// Unsubscribe 取消订阅
	Unsubscribe(ctx context.Context, topic string, handler Handler) error
}

type Payload struct {
	ID      string      `json:"id"`      // 消息ID
	Topic   string      `json:"topic"`   // 消息主题
	Message interface{} `json:"message"` // 消息内容
}

type Handler func(payload *Payload)
