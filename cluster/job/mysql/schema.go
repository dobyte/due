package mysql

// Schema returns the MySQL schema required by Store.
func Schema(tablePrefix string) string {
	if tablePrefix == "" {
		tablePrefix = "due_job"
	}

	return "CREATE TABLE " + tablePrefix + "_definition (" +
		"job_id VARCHAR(128) NOT NULL PRIMARY KEY," +
		"name VARCHAR(255) NOT NULL," +
		"task_type VARCHAR(128) NOT NULL," +
		"payload BLOB NOT NULL," +
		"schedule_type VARCHAR(32) NOT NULL," +
		"schedule_expr VARCHAR(255) NOT NULL," +
		"timezone VARCHAR(64) NOT NULL," +
		"next_run_at BIGINT NOT NULL," +
		"status VARCHAR(32) NOT NULL," +
		"misfire_policy VARCHAR(32) NOT NULL," +
		"concurrency_policy VARCHAR(32) NOT NULL," +
		"max_attempts INT NOT NULL," +
		"retry_policy VARCHAR(255) NOT NULL," +
		"timeout_ms BIGINT NOT NULL," +
		"version BIGINT NOT NULL," +
		"created_at BIGINT NOT NULL," +
		"updated_at BIGINT NOT NULL," +
		"INDEX idx_due_job_definition_due (status, next_run_at)" +
		") ENGINE=InnoDB;" +
		"CREATE TABLE " + tablePrefix + "_run (" +
		"run_id VARCHAR(128) NOT NULL PRIMARY KEY," +
		"job_id VARCHAR(128) NOT NULL," +
		"task_type VARCHAR(128) NOT NULL," +
		"payload BLOB NOT NULL," +
		"scheduled_at BIGINT NOT NULL," +
		"ready_at BIGINT NOT NULL," +
		"retry_at BIGINT NOT NULL," +
		"timeout_ms BIGINT NOT NULL," +
		"status VARCHAR(32) NOT NULL," +
		"attempt INT NOT NULL," +
		"concurrency_policy VARCHAR(32) NOT NULL," +
		"max_attempts INT NOT NULL," +
		"retry_policy VARCHAR(255) NOT NULL," +
		"lease_owner VARCHAR(128) NOT NULL," +
		"lease_until BIGINT NOT NULL," +
		"fencing_token BIGINT NOT NULL," +
		"started_at BIGINT NOT NULL," +
		"finished_at BIGINT NOT NULL," +
		"error_class VARCHAR(32) NOT NULL," +
		"error_code VARCHAR(128) NOT NULL," +
		"error_message TEXT NOT NULL," +
		"result_ref VARCHAR(512) NOT NULL," +
		"output BLOB NOT NULL," +
		"created_at BIGINT NOT NULL," +
		"updated_at BIGINT NOT NULL," +
		"UNIQUE KEY uk_due_job_run_schedule (job_id, scheduled_at)," +
		"INDEX idx_due_job_run_ready (status, ready_at)," +
		"INDEX idx_due_job_run_lease (status, lease_until)" +
		") ENGINE=InnoDB;" +
		"CREATE TABLE " + tablePrefix + "_history LIKE " + tablePrefix + "_run;"
}
