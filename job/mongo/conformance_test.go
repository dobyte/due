package mongo

import (
	"context"
	"os"
	"testing"

	"github.com/dobyte/due/v2/job"
	"github.com/dobyte/due/v2/job/conformance"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestConformance(t *testing.T) {
	uri, ok := os.LookupEnv("JOB_MONGO_URI")
	if !ok || uri == "" {
		t.Skip("set JOB_MONGO_URI to run MongoDB conformance tests")
	}
	databaseName := os.Getenv("JOB_MONGO_DATABASE")
	if databaseName == "" {
		databaseName = "due_job_conformance"
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping MongoDB: %v", err)
	}

	conformance.Run(t, func(ctx context.Context, name string) (job.JobStore, error) {
		store := NewStore(WithDB(client.Database(databaseName)), WithCollectionPrefix(name))
		if err := store.EnsureIndexes(ctx); err != nil {
			return nil, err
		}
		return store, nil
	})
}
