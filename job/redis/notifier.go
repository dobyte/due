package redis

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/job"
	"github.com/redis/go-redis/v9"
)

var _ job.EventNotifier = (*PubSubNotifier)(nil)

// PubSubNotifier broadcasts job wake-up events through Redis Pub/Sub.
type PubSubNotifier struct {
	client  redis.UniversalClient
	pubsub  *redis.PubSub
	channel string
	builtin bool

	done     chan struct{}
	loopDone chan struct{}
	events   chan string
	once     sync.Once
	mu       sync.RWMutex
	closed   bool
}

// NewNotifier creates a Redis Pub/Sub notifier.
func NewNotifier(ctx context.Context, opts ...Option) (*PubSubNotifier, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	client, builtin := clientFromOptions(ctx, o)
	channel := o.prefix + ":notify"
	pubsub := client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		if builtin {
			_ = client.Close()
		}
		return nil, err
	}

	notifier := &PubSubNotifier{
		client:   client,
		pubsub:   pubsub,
		channel:  channel,
		builtin:  builtin,
		done:     make(chan struct{}),
		loopDone: make(chan struct{}),
		events:   make(chan string, 128),
	}
	go notifier.receiveLoop()
	return notifier, nil
}

// Notify publishes a job ID to all subscribed workers.
func (n *PubSubNotifier) Notify(ctx context.Context, jobID string) error {
	if n == nil || jobID == "" {
		return errors.ErrInvalidArgument
	}
	if ctx == nil {
		ctx = context.Background()
	}
	n.mu.RLock()
	closed := n.closed
	n.mu.RUnlock()
	if closed {
		return job.ErrJobClosed
	}
	return n.client.Publish(ctx, n.channel, jobID).Err()
}

// Events returns the received job IDs.
func (n *PubSubNotifier) Events() <-chan string {
	if n == nil {
		return nil
	}
	return n.events
}

// Close unsubscribes and closes the notifier.
func (n *PubSubNotifier) Close(context.Context) error {
	if n == nil {
		return nil
	}
	n.once.Do(func() {
		n.mu.Lock()
		n.closed = true
		n.mu.Unlock()
		close(n.done)
		_ = n.pubsub.Close()
		<-n.loopDone
		if n.builtin {
			_ = n.client.Close()
		}
		close(n.events)
	})
	return nil
}

func (n *PubSubNotifier) receiveLoop() {
	defer close(n.loopDone)
	for {
		message, err := n.pubsub.ReceiveMessage(context.Background())
		if err != nil {
			return
		}
		select {
		case n.events <- message.Payload:
		case <-n.done:
			return
		}
	}
}
