package eventbus

import (
	"context"
	"github.com/dobyte/due/encoding/json"
	"github.com/dobyte/due/internal/value"
	"github.com/dobyte/due/utils/xconv"
	"github.com/dobyte/due/utils/xtime"
	"github.com/dobyte/due/utils/xuuid"
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

type packet struct {
	ID        string `json:"id"`        // 事件ID
	Topic     string `json:"topic"`     // 事件主题
	Payload   string `json:"payload"`   // 事件载荷
	Timestamp int64  `json:"timestamp"` // 事件时间
}

// BuildPayload 构建
func BuildPayload(topic string, payload interface{}) ([]byte, error) {
	id, err := xuuid.UUID()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&packet{
		ID:        id,
		Topic:     topic,
		Payload:   xconv.String(payload),
		Timestamp: xtime.Now().UnixNano(),
	})
}

// ParsePayload 解析
func ParsePayload(payload string) (*Event, error) {
	p := &packet{}

	err := json.Unmarshal(xconv.Bytes(payload), p)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:        p.ID,
		Topic:     p.Topic,
		Payload:   value.NewValue(p.Payload),
		Timestamp: xtime.UnixNano(p.Timestamp),
	}, nil
}
