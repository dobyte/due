package process

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/eventbus"
)

const testTopic = "test-topic"

// countingHandler returns an event handler that increments counter and signals ch.
func countingHandler(ch chan<- struct{}, counter *int32) eventbus.EventHandler {
	return func(*eventbus.Event) {
		atomic.AddInt32(counter, 1)
		ch <- struct{}{}
	}
}

// waitSignal waits for a signal on ch and reports whether it arrived before the timeout.
func waitSignal(t *testing.T, ch <-chan struct{}) bool {
	t.Helper()

	select {
	case <-ch:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

// TestNewEventbus verifies that NewEventbus initializes the consumers map.
func TestNewEventbus(t *testing.T) {
	eb := NewEventbus()
	if eb == nil {
		t.Fatal("NewEventbus() = nil, want non-nil")
	}
	if eb.consumers == nil {
		t.Error("NewEventbus() consumers = nil, want non-nil")
	}
}

// TestEventbusPublish verifies publishing to unknown and known topics.
func TestEventbusPublish(t *testing.T) {
	tests := []struct {
		name  string
		topic string
		found bool
	}{
		{name: "unknown topic is ignored", topic: "unknown", found: false},
		{name: "known topic dispatches", topic: testTopic, found: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := NewEventbus()
			ch := make(chan struct{}, 1)
			var counter int32

			if tt.found {
				if _, err := eb.Subscribe(context.Background(), tt.topic, countingHandler(ch, &counter)); err != nil {
					t.Fatalf("Subscribe() = %v, want nil", err)
				}
			}

			if err := eb.Publish(context.Background(), tt.topic, "payload"); err != nil {
				t.Fatalf("Publish() = %v, want nil", err)
			}

			if !tt.found {
				select {
				case <-ch:
					t.Error("handler invoked for unknown topic")
				case <-time.After(50 * time.Millisecond):
				}
				return
			}

			if !waitSignal(t, ch) {
				t.Fatal("handler was not invoked")
			}
			if got := atomic.LoadInt32(&counter); got != 1 {
				t.Errorf("handler invocations = %d, want 1", got)
			}
		})
	}
}

// TestEventbusSubscribe verifies the consumer creation and balance conflict branches.
func TestEventbusSubscribe(t *testing.T) {
	handler := func(*eventbus.Event) {}

	tests := []struct {
		name    string
		first   []bool
		second  []bool
		wantErr error
	}{
		{name: "creates consumer for new topic", first: nil, second: nil, wantErr: nil},
		{name: "reuses consumer with matching balance", first: []bool{true}, second: []bool{true}, wantErr: nil},
		{name: "conflicts with existing balance", first: []bool{true}, second: []bool{false}, wantErr: errors.ErrInvalidArgument},
		{name: "omitted balance matches existing", first: []bool{true}, second: nil, wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := NewEventbus()

			if _, err := eb.Subscribe(context.Background(), testTopic, handler, tt.first...); err != nil {
				t.Fatalf("first Subscribe() = %v, want nil", err)
			}

			sub, err := eb.Subscribe(context.Background(), testTopic, handler, tt.second...)
			if err != tt.wantErr {
				t.Errorf("second Subscribe() = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && sub == nil {
				t.Error("second Subscribe() = nil, want non-nil")
			}
			if tt.wantErr == nil && sub != nil {
				if err := sub.Unsubscribe(context.Background()); err != nil {
					t.Errorf("Unsubscribe() = %v, want nil", err)
				}
			}
		})
	}
}

// TestEventbusSubscribeCreatesConsumer verifies that the consumer map is populated on subscribe.
func TestEventbusSubscribeCreatesConsumer(t *testing.T) {
	eb := NewEventbus()

	if _, err := eb.Subscribe(context.Background(), testTopic, func(*eventbus.Event) {}); err != nil {
		t.Fatalf("Subscribe() = %v, want nil", err)
	}

	eb.rw.RLock()
	_, ok := eb.consumers[testTopic]
	eb.rw.RUnlock()

	if !ok {
		t.Errorf("consumers[%q] not found, want present", testTopic)
	}
}

// TestEventbusUnsubscribe verifies the removal, repeat and foreign-subscription branches.
func TestEventbusUnsubscribe(t *testing.T) {
	handler := func(*eventbus.Event) {}

	tests := []struct {
		name   string
		action func(t *testing.T, eb *Eventbus)
	}{
		{
			name: "removes empty consumer",
			action: func(t *testing.T, eb *Eventbus) {
				sub, err := eb.Subscribe(context.Background(), testTopic, handler)
				if err != nil {
					t.Fatalf("Subscribe() = %v, want nil", err)
				}
				if err := sub.Unsubscribe(context.Background()); err != nil {
					t.Errorf("Unsubscribe() = %v, want nil", err)
				}

				eb.rw.RLock()
				_, ok := eb.consumers[testTopic]
				eb.rw.RUnlock()
				if ok {
					t.Error("consumer was not removed after the last unsubscribe")
				}
			},
		},
		{
			name: "unsubscribe twice returns illegal operation",
			action: func(t *testing.T, eb *Eventbus) {
				sub, err := eb.Subscribe(context.Background(), testTopic, handler)
				if err != nil {
					t.Fatalf("Subscribe() = %v, want nil", err)
				}
				if err := sub.Unsubscribe(context.Background()); err != nil {
					t.Fatalf("first Unsubscribe() = %v, want nil", err)
				}
				if err := sub.Unsubscribe(context.Background()); err != errors.ErrIllegalOperation {
					t.Errorf("second Unsubscribe() = %v, want %v", err, errors.ErrIllegalOperation)
				}
			},
		},
		{
			name: "foreign subscription returns illegal operation",
			action: func(t *testing.T, eb *Eventbus) {
				if _, err := eb.Subscribe(context.Background(), testTopic, handler); err != nil {
					t.Fatalf("Subscribe() = %v, want nil", err)
				}
				foreign := &subscription{topic: testTopic}
				if err := eb.unsubscribe(testTopic, foreign); err != errors.ErrIllegalOperation {
					t.Errorf("unsubscribe() = %v, want %v", err, errors.ErrIllegalOperation)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := NewEventbus()
			tt.action(t, eb)
		})
	}
}

// TestEventbusClose verifies that Close returns nil.
func TestEventbusClose(t *testing.T) {
	if err := NewEventbus().Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// TestConsumerAddDelSubscription verifies adding and removing subscriptions.
func TestConsumerAddDelSubscription(t *testing.T) {
	c := &consumer{}
	sub1 := c.addSubscription(testTopic, func(*eventbus.Event) {})
	sub2 := c.addSubscription(testTopic, func(*eventbus.Event) {})

	if got := len(c.subscriptions); got != 2 {
		t.Fatalf("subscriptions = %d, want 2", got)
	}

	found, empty := c.delSubscription(sub1)
	if !found {
		t.Error("delSubscription(sub1) found = false, want true")
	}
	if empty {
		t.Error("delSubscription(sub1) empty = true, want false")
	}

	found, empty = c.delSubscription(sub2)
	if !found {
		t.Error("delSubscription(sub2) found = false, want true")
	}
	if !empty {
		t.Error("delSubscription(sub2) empty = false, want true")
	}

	found, _ = c.delSubscription(&subscription{})
	if found {
		t.Error("delSubscription(foreign) found = true, want false")
	}
}

// TestConsumerDispatch verifies the fanout and balance dispatch paths.
func TestConsumerDispatch(t *testing.T) {
	tests := []struct {
		name        string
		balance     bool
		handlers    []bool // whether each subscription carries a non-nil handler
		wantInvoked int32
	}{
		{name: "fanout invokes all handlers", balance: false, handlers: []bool{true, true, true}, wantInvoked: 3},
		{name: "fanout skips nil handlers", balance: false, handlers: []bool{true, false, true}, wantInvoked: 2},
		{name: "balance invokes exactly one handler", balance: true, handlers: []bool{true, true, true}, wantInvoked: 1},
		{name: "balance with no handler is a no-op", balance: true, handlers: []bool{false}, wantInvoked: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := make(chan struct{}, len(tt.handlers)+1)
			var counter int32

			c := &consumer{balance: tt.balance}
			for _, hasHandler := range tt.handlers {
				if hasHandler {
					c.addSubscription(testTopic, countingHandler(ch, &counter))
				} else {
					c.addSubscription(testTopic, nil)
				}
			}

			c.dispatch(&eventbus.Event{Topic: testTopic})

			if tt.wantInvoked == 0 {
				select {
				case <-ch:
					t.Error("handler invoked, want none")
				case <-time.After(50 * time.Millisecond):
				}
				return
			}

			for i := int32(0); i < tt.wantInvoked; i++ {
				if !waitSignal(t, ch) {
					t.Fatalf("handler %d was not invoked", i)
				}
			}

			if got := atomic.LoadInt32(&counter); got != tt.wantInvoked {
				t.Errorf("handler invocations = %d, want %d", got, tt.wantInvoked)
			}

			if tt.balance {
				select {
				case <-ch:
					t.Error("more than one handler invoked in balance mode")
				case <-time.After(50 * time.Millisecond):
				}
			}
		})
	}
}

// TestConsumerDispatchEmptyBalance verifies that a balance consumer without subscriptions is a no-op.
func TestConsumerDispatchEmptyBalance(t *testing.T) {
	c := &consumer{balance: true}
	c.dispatch(&eventbus.Event{Topic: testTopic})
}

// TestSubscriptionUnsubscribe verifies that a subscription delegates to its eventbus.
func TestSubscriptionUnsubscribe(t *testing.T) {
	eb := NewEventbus()
	sub, err := eb.Subscribe(context.Background(), testTopic, func(*eventbus.Event) {})
	if err != nil {
		t.Fatalf("Subscribe() = %v, want nil", err)
	}

	if err := sub.Unsubscribe(context.Background()); err != nil {
		t.Errorf("Unsubscribe() = %v, want nil", err)
	}

	eb.rw.RLock()
	_, ok := eb.consumers[testTopic]
	eb.rw.RUnlock()
	if ok {
		t.Error("consumer still present after Unsubscribe")
	}
}
