package nats

import (
	"context"

	"github.com/nats-io/nats.go"
)

type subscription struct {
	sub *nats.Subscription
}

// Unsubscribe 取消订阅
func (s *subscription) Unsubscribe(_ context.Context) error {
	return s.sub.Unsubscribe()
}
