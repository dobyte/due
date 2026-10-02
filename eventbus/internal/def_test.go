package internal

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/value"
)

// TestEventHandler verifies that an EventHandler receives the published event.
func TestEventHandler(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		event *Event
	}{
		{
			name: "complete event",
			event: &Event{
				ID:        "id-1",
				Topic:     "topic-1",
				Payload:   value.NewValue("payload"),
				Timestamp: now,
			},
		},
		{
			name:  "zero event",
			event: &Event{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *Event

			var handler EventHandler = func(event *Event) {
				got = event
			}
			handler(tt.event)

			if got != tt.event {
				t.Fatalf("handler received %v, want %v", got, tt.event)
			}
			if got.ID != tt.event.ID {
				t.Errorf("Event.ID = %q, want %q", got.ID, tt.event.ID)
			}
			if got.Topic != tt.event.Topic {
				t.Errorf("Event.Topic = %q, want %q", got.Topic, tt.event.Topic)
			}
			if got.Timestamp != tt.event.Timestamp {
				t.Errorf("Event.Timestamp = %v, want %v", got.Timestamp, tt.event.Timestamp)
			}
		})
	}
}
