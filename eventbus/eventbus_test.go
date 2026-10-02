package eventbus

import (
	"context"
	"testing"

	"github.com/dobyte/due/v2/errors"
)

// mockSubscription is a stub implementation of the Subscription interface.
type mockSubscription struct {
	unsubscribeCalls int
	unsubscribeErr   error
}

// Unsubscribe records the call and returns the configured error.
func (s *mockSubscription) Unsubscribe(_ context.Context) error {
	s.unsubscribeCalls++
	return s.unsubscribeErr
}

// mockEventbus is a stub implementation of the Eventbus interface.
type mockEventbus struct {
	closeCalls     int
	closeErr       error
	publishCalls   int
	publishErr     error
	subscribeCalls int
	subscribeErr   error
	subscription   Subscription
}

// Close records the call and returns the configured error.
func (m *mockEventbus) Close() error {
	m.closeCalls++
	return m.closeErr
}

// Publish records the call and returns the configured error.
func (m *mockEventbus) Publish(_ context.Context, _ string, _ any) error {
	m.publishCalls++
	return m.publishErr
}

// Subscribe records the call and returns the configured subscription and error.
func (m *mockEventbus) Subscribe(_ context.Context, _ string, _ EventHandler, _ ...bool) (Subscription, error) {
	m.subscribeCalls++
	return m.subscription, m.subscribeErr
}

// TestGetEventbus verifies that GetEventbus returns the current global eventbus.
func TestGetEventbus(t *testing.T) {
	prev := globalEventbus
	defer func() { globalEventbus = prev }()

	tests := []struct {
		name string
		eb   Eventbus
	}{
		{name: "nil", eb: nil},
		{name: "non-nil", eb: &mockEventbus{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalEventbus = tt.eb
			if got := GetEventbus(); got != tt.eb {
				t.Errorf("GetEventbus() = %v, want %v", got, tt.eb)
			}
		})
	}
}

// TestSetEventbus verifies the nil-guard, replacement and close-error branches of SetEventbus.
func TestSetEventbus(t *testing.T) {
	prev := globalEventbus
	defer func() { globalEventbus = prev }()

	tests := []struct {
		name    string
		prepare func() Eventbus
		arg     func(initial Eventbus) Eventbus
		assert  func(t *testing.T, initial, current Eventbus)
	}{
		{
			name:    "nil eventbus is ignored",
			prepare: func() Eventbus { return nil },
			arg:     func(Eventbus) Eventbus { return nil },
			assert: func(t *testing.T, _, current Eventbus) {
				if current != nil {
					t.Errorf("SetEventbus(nil) global = %v, want nil", current)
				}
			},
		},
		{
			name:    "sets eventbus when none exists",
			prepare: func() Eventbus { return nil },
			arg:     func(Eventbus) Eventbus { return &mockEventbus{} },
			assert: func(t *testing.T, _, current Eventbus) {
				if current == nil {
					t.Error("SetEventbus() global = nil, want non-nil")
				}
			},
		},
		{
			name:    "replaces and closes previous eventbus",
			prepare: func() Eventbus { return &mockEventbus{} },
			arg:     func(Eventbus) Eventbus { return &mockEventbus{} },
			assert: func(t *testing.T, initial, current Eventbus) {
				if initial == current {
					t.Error("SetEventbus() did not replace the previous eventbus")
				}
				if got := initial.(*mockEventbus).closeCalls; got != 1 {
					t.Errorf("previous close calls = %d, want 1", got)
				}
			},
		},
		{
			name:    "close error of previous is logged but ignored",
			prepare: func() Eventbus { return &mockEventbus{closeErr: errors.New("close failed")} },
			arg:     func(Eventbus) Eventbus { return &mockEventbus{} },
			assert: func(t *testing.T, initial, current Eventbus) {
				if got := initial.(*mockEventbus).closeCalls; got != 1 {
					t.Errorf("previous close calls = %d, want 1", got)
				}
				if current == nil {
					t.Error("SetEventbus() global = nil, want non-nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial := tt.prepare()
			globalEventbus = initial
			SetEventbus(tt.arg(initial))
			tt.assert(t, initial, globalEventbus)
		})
	}
}

// TestPublish verifies the missing-instance branch and delegation of Publish.
func TestPublish(t *testing.T) {
	prev := globalEventbus
	defer func() { globalEventbus = prev }()

	tests := []struct {
		name    string
		eb      Eventbus
		wantErr error
	}{
		{name: "missing instance", eb: nil, wantErr: errors.ErrMissingEventbusInstance},
		{name: "delegates to instance", eb: &mockEventbus{}, wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalEventbus = tt.eb
			err := Publish(context.Background(), "topic", "message")
			if err != tt.wantErr {
				t.Errorf("Publish() = %v, want %v", err, tt.wantErr)
			}
			if tt.eb != nil {
				if got := tt.eb.(*mockEventbus).publishCalls; got != 1 {
					t.Errorf("publish calls = %d, want 1", got)
				}
			}
		})
	}
}

// TestSubscribe verifies the missing-instance branch and delegation of Subscribe.
func TestSubscribe(t *testing.T) {
	prev := globalEventbus
	defer func() { globalEventbus = prev }()

	tests := []struct {
		name    string
		eb      Eventbus
		wantErr error
	}{
		{name: "missing instance", eb: nil, wantErr: errors.ErrMissingEventbusInstance},
		{name: "delegates to instance", eb: &mockEventbus{subscription: &mockSubscription{}}, wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalEventbus = tt.eb
			sub, err := Subscribe(context.Background(), "topic", func(*Event) {})
			if err != tt.wantErr {
				t.Errorf("Subscribe() error = %v, want %v", err, tt.wantErr)
			}
			if tt.eb == nil && sub != nil {
				t.Errorf("Subscribe() = %v, want nil", sub)
			}
			if tt.eb != nil {
				if got := tt.eb.(*mockEventbus).subscribeCalls; got != 1 {
					t.Errorf("subscribe calls = %d, want 1", got)
				}
			}
		})
	}
}

// TestClose verifies the nil-guard and delegation branches of Close.
func TestClose(t *testing.T) {
	prev := globalEventbus
	defer func() { globalEventbus = prev }()

	tests := []struct {
		name    string
		eb      Eventbus
		wantErr error
	}{
		{name: "missing instance", eb: nil, wantErr: nil},
		{name: "delegates to instance", eb: &mockEventbus{closeErr: errors.New("close failed")}, wantErr: errors.New("close failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalEventbus = tt.eb
			err := Close()
			if (err == nil) != (tt.wantErr == nil) {
				t.Errorf("Close() = %v, want %v", err, tt.wantErr)
			}
			if tt.eb != nil {
				if got := tt.eb.(*mockEventbus).closeCalls; got != 1 {
					t.Errorf("close calls = %d, want 1", got)
				}
			}
		})
	}
}
