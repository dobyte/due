package polaris

import (
	"context"
	"testing"
	"time"
)

// TestSourceRetrySubscriptions verifies that only the configured exact files without an active
// subscription are queued again when group search is unavailable.
func TestSourceRetrySubscriptions(t *testing.T) {
	o := defaultOptions()
	o.files = []string{"a.json", "b.yaml", "*.json"}

	source := &Source{opts: o, chListen: make(chan string, 2)}
	source.ctx, source.cancel = context.WithCancel(context.Background())
	defer source.cancel()

	source.subscribed.Store("a.json", struct{}{})

	source.retrySubscriptions()

	select {
	case fileName := <-source.chListen:
		if fileName != "b.yaml" {
			t.Fatalf("unexpected listen request: %s", fileName)
		}
	case <-time.After(time.Second):
		t.Fatal("the missing file is not queued for subscription")
	}

	select {
	case fileName := <-source.chListen:
		t.Fatalf("unexpected listen request: %s", fileName)
	case <-time.After(50 * time.Millisecond):
	}
}
