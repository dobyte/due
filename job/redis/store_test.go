package redis

import (
	stderrors "errors"
	"testing"
	"time"

	"github.com/dobyte/due/v2/job"
)

func TestFieldArgumentCounts(t *testing.T) {
	definition := &job.Definition{ID: "job", Type: "test"}
	if got := len(definitionFields(definition)); got != 34 {
		t.Fatalf("definition field count = %d, want 34", got)
	}

	run := &job.Run{ID: "run", JobID: "job", TaskType: "test"}
	if got := len(runFields(run)); got != 46 {
		t.Fatalf("run field count = %d, want 46", got)
	}
}

func TestNextSchedule(t *testing.T) {
	now := time.UnixMilli(1000)
	definition := &job.Definition{
		ScheduleType: job.ScheduleFixed,
		ScheduleExpr: "1s",
	}

	next, status, err := job.NextSchedule(definition, now)
	if err != nil {
		t.Fatalf("nextSchedule() error = %v", err)
	}
	if !next.Equal(now.Add(time.Second)) {
		t.Fatalf("next schedule = %v, want %v", next, now.Add(time.Second))
	}
	if status != job.DefinitionEnabled {
		t.Fatalf("next status = %q, want %q", status, job.DefinitionEnabled)
	}
}

func TestNextScheduleRejectsInvalidCron(t *testing.T) {
	_, _, err := job.NextSchedule(&job.Definition{
		ScheduleType: job.ScheduleCron,
		ScheduleExpr: "invalid",
	}, time.Now())
	if !stderrors.Is(err, job.ErrInvalidSchedule) {
		t.Fatalf("NextSchedule() error = %v, want ErrInvalidSchedule", err)
	}
}
