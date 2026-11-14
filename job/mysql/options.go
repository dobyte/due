package mysql

import "database/sql"

type Option func(*options)

type options struct {
	db          *sql.DB
	tablePrefix string
	skipLocked  bool
}

func defaultOptions() *options {
	return &options{
		tablePrefix: "due_job",
		skipLocked:  true,
	}
}

func WithDB(db *sql.DB) Option {
	return func(o *options) { o.db = db }
}

func WithTablePrefix(prefix string) Option {
	return func(o *options) {
		if prefix != "" {
			o.tablePrefix = prefix
		}
	}
}

func WithSkipLocked(enabled bool) Option {
	return func(o *options) { o.skipLocked = enabled }
}
