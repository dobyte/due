package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster/job"
	"github.com/dobyte/due/v2/cluster/job/conformance"
)

func TestNotifier(t *testing.T) {
	addr, ok := os.LookupEnv("JOB_REDIS_ADDR")
	if !ok || addr == "" {
		t.Skip("set JOB_REDIS_ADDR to run Redis notifier tests")
	}
	notifier, err := NewNotifier(context.Background(), WithAddrs(addr), WithPrefix("due:job:notifier:{conformance}"))
	if err != nil {
		t.Fatalf("create notifier: %v", err)
	}
	defer notifier.Close(context.Background())
	if err := notifier.Notify(context.Background(), "job-1"); err != nil {
		t.Fatalf("notify: %v", err)
	}
	select {
	case jobID := <-notifier.Events():
		if jobID != "job-1" {
			t.Fatalf("job ID = %q, want job-1", jobID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notifier event")
	}
}

func TestConformance(t *testing.T) {
	addr, ok := os.LookupEnv("JOB_REDIS_ADDR")
	if !ok || addr == "" {
		t.Skip("set JOB_REDIS_ADDR to run Redis conformance tests")
	}

	conformance.Run(t, func(ctx context.Context, name string) (job.JobStore, error) {
		return NewStore(
			WithAddrs(addr),
			WithPrefix("due:job:conformance:{"+name+"}"),
		), nil
	})
}
