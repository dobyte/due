package eventbus

import (
	"context"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/eventbus/internal"
	"github.com/dobyte/due/v2/log"
)

var globalEventbus Eventbus

type (
	Event        = internal.Event
	EventHandler = internal.EventHandler
)

type Eventbus interface {
	// Close closes the eventbus.
	Close() error
	// Publish publishes an event.
	Publish(ctx context.Context, topic string, message any) error
	// Subscribe subscribes to an event.
	Subscribe(ctx context.Context, topic string, handler EventHandler, balance ...bool) (Subscription, error)
}

type Subscription interface {
	// Unsubscribe cancels the subscription.
	Unsubscribe(ctx context.Context) error
}

// SetEventbus sets the eventbus.
func SetEventbus(eb Eventbus) {
	if eb == nil {
		log.Warn("cannot set a nil eventbus")
		return
	}

	if globalEventbus != nil {
		if err := globalEventbus.Close(); err != nil {
			log.Errorf("the old eventbus close failed: %v", err)
		}
	}

	globalEventbus = eb
}

// GetEventbus returns the eventbus.
func GetEventbus() Eventbus {
	return globalEventbus
}

// Publish publishes an event.
func Publish(ctx context.Context, topic string, message any) error {
	if globalEventbus == nil {
		return errors.ErrMissingEventbusInstance
	}

	return globalEventbus.Publish(ctx, topic, message)
}

// Subscribe subscribes to an event.
func Subscribe(ctx context.Context, topic string, handler EventHandler, balance ...bool) (Subscription, error) {
	if globalEventbus == nil {
		return nil, errors.ErrMissingEventbusInstance
	}

	return globalEventbus.Subscribe(ctx, topic, handler, balance...)
}

// Close closes the eventbus.
func Close() error {
	if globalEventbus == nil {
		return nil
	}

	return globalEventbus.Close()
}
