package process

import (
	"context"

	"github.com/dobyte/due/v2/eventbus"
)

type subscription struct {
	eb      *Eventbus
	topic   string
	handler eventbus.EventHandler
}

// Unsubscribe 取消订阅
func (s *subscription) Unsubscribe(_ context.Context) error {
	return s.eb.unsubscribe(s.topic, s)
}
