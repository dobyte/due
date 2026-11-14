package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/dobyte/due/v2/job"
	"github.com/dobyte/due/v2/job/conformance"
	_ "github.com/go-sql-driver/mysql"
)

func TestConformance(t *testing.T) {
	dsn, ok := os.LookupEnv("JOB_MYSQL_DSN")
	if !ok || dsn == "" {
		t.Skip("set JOB_MYSQL_DSN to run MySQL conformance tests")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	conformance.Run(t, func(ctx context.Context, name string) (job.JobStore, error) {
		prefix := strings.ReplaceAll(name, "-", "_")
		if _, err := db.ExecContext(ctx, Schema(prefix)); err != nil {
			return nil, err
		}
		return NewStore(WithDB(db), WithTablePrefix(prefix)), nil
	})
}
