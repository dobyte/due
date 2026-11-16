package job

import (
	"testing"
	"time"
)

func TestNextScheduleCron(t *testing.T) {
	definition := &Definition{
		ScheduleType: ScheduleCron,
		ScheduleExpr: "0 9 * * *",
		Timezone:     "Asia/Shanghai",
	}
	start := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)

	next, status, err := NextSchedule(definition, start)
	if err != nil {
		t.Fatalf("NextSchedule() error = %v", err)
	}
	if status != DefinitionEnabled {
		t.Fatalf("status = %q, want %q", status, DefinitionEnabled)
	}
	want := time.Date(2026, 11, 15, 1, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next = %v, want %v", next, want)
	}
}
