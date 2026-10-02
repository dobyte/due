package nats

import (
	"context"

	"github.com/nats-io/nats.go"
)

type subscription struct {
	sub *nats.Subscription
}

// Unsubscribe cancels the subscription.
func (s *subscription) Unsubscribe(_ context.Context) error {
	return s.sub.Unsubscribe()
}
