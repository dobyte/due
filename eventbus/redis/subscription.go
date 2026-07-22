package redis

import (
	"context"

	"github.com/dobyte/due/v2/eventbus"
)

type subscription struct {
	eb       *Eventbus
	topic    string
	handler  eventbus.EventHandler
	single   bool
	consumer *consumer
	stream   string
	group    string
}

// Unsubscribe 取消订阅
func (s *subscription) Unsubscribe(ctx context.Context) error {
	s.eb.unsubscribe(ctx, s)
	return nil
}
