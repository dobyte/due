package mongo

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/job"
)

func TestDocumentRoundTrip(t *testing.T) {
	definition := &job.Definition{
		ID: "daily", Type: "mail", Payload: []byte("x"), ScheduleType: job.ScheduleCron,
		ScheduleExpr: "0 9 * * *", Timezone: "Asia/Shanghai", NextRunAt: time.Now(),
		Status: job.DefinitionEnabled, MisfirePolicy: job.MisfireFireOnce,
		ConcurrencyPolicy: job.ForbidConcurrent, MaxAttempts: 3,
		RetryPolicy: job.DefaultRetryPolicy(), Timeout: 2 * time.Second, Version: 1,
	}
	doc := toDefinitionDocument(definition)
	roundTrip := fromDefinitionDocument(&doc)
	if roundTrip.ID != definition.ID || roundTrip.Type != definition.Type || roundTrip.ScheduleExpr != definition.ScheduleExpr || roundTrip.Timeout != definition.Timeout {
		t.Fatalf("definition round trip mismatch: %#v", roundTrip)
	}

	run := &job.Run{ID: "run-1", JobID: definition.ID, TaskType: definition.Type, Payload: []byte("p"), ScheduledAt: time.Now(), ReadyAt: time.Now(), Status: job.RunPending, RetryPolicy: job.DefaultRetryPolicy(), Output: []byte("o")}
	runDoc := toRunDocument(run)
	runRoundTrip := fromRunDocument(&runDoc)
	if runRoundTrip.ID != run.ID || runRoundTrip.JobID != run.JobID || string(runRoundTrip.Output) != string(run.Output) {
		t.Fatalf("run round trip mismatch: %#v", runRoundTrip)
	}
}

func TestNilStoreRejectsOperations(t *testing.T) {
	store := NewStore()
	if err := store.EnsureIndexes(t.Context()); err == nil {
		t.Fatal("expected nil store error")
	}
}
