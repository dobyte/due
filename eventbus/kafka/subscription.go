package kafka

import (
	"context"

	"github.com/dobyte/due/v2/eventbus"
)

type subscription struct {
	eb       *Eventbus
	topic    string
	consumer *consumer
	handler  eventbus.EventHandler
}

func (s *subscription) Unsubscribe(_ context.Context) error {
	s.eb.unsubscribe(s)
	return nil
}
