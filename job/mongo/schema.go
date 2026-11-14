package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongooptions "go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes creates the indexes used by Store. It is safe to call at startup.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	if err := s.check(); err != nil {
		return err
	}
	_, err := s.definitions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "next_run_at", Value: 1}}},
	})
	if err != nil {
		return err
	}
	_, err = s.runs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "job_id", Value: 1}, {Key: "scheduled_at", Value: 1}}, Options: mongooptions.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "ready_at", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "retry_at", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "lease_until", Value: 1}}},
	})
	return err
}
