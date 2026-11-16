package mysql

import "testing"

func TestSchemaContainsCoreTablesAndIndexes(t *testing.T) {
	schema := Schema("demo_job")
	for _, fragment := range []string{
		"CREATE TABLE demo_job_definition",
		"CREATE TABLE demo_job_run",
		"UNIQUE KEY uk_due_job_run_schedule",
		"idx_due_job_definition_due",
		"idx_due_job_run_lease",
	} {
		if !contains(schema, fragment) {
			t.Fatalf("schema missing %q", fragment)
		}
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
