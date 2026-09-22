package kafka

import (
	"context"

	"github.com/dobyte/due/v2/eventbus"
)

// subscription 事件订阅
type subscription struct {
	eb       *Eventbus
	topic    string
	consumer *consumer
	handler  eventbus.EventHandler
}

// Unsubscribe 取消订阅
func (s *subscription) Unsubscribe(_ context.Context) error {
	s.eb.unsubscribe(s)
	return nil
}
