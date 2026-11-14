package mongo

import "go.mongodb.org/mongo-driver/mongo"

type Option func(*options)

type options struct {
	db             *mongo.Database
	definitionColl string
	runColl        string
}

func defaultOptions() *options {
	return &options{definitionColl: "due_job_definition", runColl: "due_job_run"}
}

func WithDB(db *mongo.Database) Option { return func(o *options) { o.db = db } }

func WithCollectionPrefix(prefix string) Option {
	return func(o *options) {
		if prefix != "" {
			o.definitionColl = prefix + "_definition"
			o.runColl = prefix + "_run"
		}
	}
}

func WithCollections(definition, run string) Option {
	return func(o *options) {
		if definition != "" {
			o.definitionColl = definition
		}
		if run != "" {
			o.runColl = run
		}
	}
}
