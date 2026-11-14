package job

import (
	"context"
	"testing"
)

func TestChannelNotifier(t *testing.T) {
	notifier := NewChannelNotifier(1)
	if err := notifier.Notify(context.Background(), "job-1"); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	select {
	case jobID := <-notifier.Events():
		if jobID != "job-1" {
			t.Fatalf("job ID = %q, want job-1", jobID)
		}
	default:
		t.Fatal("expected notification")
	}
	if err := notifier.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := notifier.Notify(context.Background(), "job-2"); err != ErrJobClosed {
		t.Fatalf("Notify() after close = %v, want ErrJobClosed", err)
	}
}

func TestNopMetrics(t *testing.T) {
	var metrics Metrics = NopMetrics{}
	metrics.Add("test", 1)
	metrics.Observe("test_duration", 0.1)
}
