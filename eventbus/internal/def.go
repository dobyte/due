package internal

import (
	"time"

	"github.com/dobyte/due/v2/core/value"
)

type EventHandler func(event *Event)

type Event struct {
	ID        string      // Event ID
	Topic     string      // Event topic
	Payload   value.Value // Event payload
	Timestamp time.Time   // Event timestamp
}
