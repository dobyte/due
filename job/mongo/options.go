package mongo

import "go.mongodb.org/mongo-driver/mongo"

type Option func(*storeOptions)

type storeOptions struct {
	db             *mongo.Database
	definitionColl string
	runColl        string
}

func defaultOptions() *storeOptions {
	return &storeOptions{definitionColl: "due_job_definition", runColl: "due_job_run"}
}

func WithDB(db *mongo.Database) Option { return func(o *storeOptions) { o.db = db } }

func WithCollectionPrefix(prefix string) Option {
	return func(o *storeOptions) {
		if prefix != "" {
			o.definitionColl = prefix + "_definition"
			o.runColl = prefix + "_run"
		}
	}
}

func WithCollections(definition, run string) Option {
	return func(o *storeOptions) {
		if definition != "" {
			o.definitionColl = definition
		}
		if run != "" {
			o.runColl = run
		}
	}
}
