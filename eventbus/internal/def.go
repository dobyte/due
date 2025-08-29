package internal

import (
	"time"

	"github.com/dobyte/due/v2/core/value"
)

type EventHandler func(event *Event)

type Event struct {
	ID        string      // 事件ID
	Topic     string      // 事件主题
	Payload   value.Value // 事件载荷
	Timestamp time.Time   // 事件时间
}
