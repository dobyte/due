package redis

import (
	"context"
	"os"
	"testing"

	"github.com/dobyte/due/v2/job"
	"github.com/dobyte/due/v2/job/conformance"
)

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
