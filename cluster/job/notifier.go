package job

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/errors"
)

// EventNotifier is a Notifier that can wake a Service immediately.
type EventNotifier interface {
	Notifier
	Events() <-chan string
}

// ChannelNotifier is an in-process notifier useful for single-process workers
// and as an adapter boundary for external pub/sub implementations.
type ChannelNotifier struct {
	mu     sync.RWMutex
	ch     chan string
	closed bool
}

// NewChannelNotifier creates a buffered in-process notifier.
func NewChannelNotifier(buffer int) *ChannelNotifier {
	if buffer <= 0 {
		buffer = 1
	}
	return &ChannelNotifier{ch: make(chan string, buffer)}
}

// Notify publishes a job ID wake-up event.
func (n *ChannelNotifier) Notify(ctx context.Context, jobID string) error {
	if n == nil || jobID == "" {
		return errors.ErrInvalidArgument
	}
	if ctx == nil {
		ctx = context.Background()
	}

	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.closed {
		return ErrJobClosed
	}

	select {
	case n.ch <- jobID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Events returns the wake-up channel.
func (n *ChannelNotifier) Events() <-chan string {
	if n == nil {
		return nil
	}
	return n.ch
}

// Close stops future notifications and closes the event channel.
func (n *ChannelNotifier) Close(context.Context) error {
	if n == nil {
		return nil
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return nil
	}
	n.closed = true
	close(n.ch)
	return nil
}
