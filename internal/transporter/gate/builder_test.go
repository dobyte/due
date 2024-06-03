package gate_test

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/utils/xuuid"
	"testing"
)

func TestBuilder(t *testing.T) {
	builder := gate.NewBuilder(&gate.Options{
		InsID:   xuuid.UUID(),
		InsKind: cluster.Node,
	})

	client, err := builder.Build("127.0.0.1:49899")
	if err != nil {
		t.Fatal(err)
	}

	ip, miss, err := client.GetIP(context.Background(), session.User, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("miss: %v ip: %v", miss, ip)

	ip, miss, err = client.GetIP(context.Background(), session.User, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("miss: %v ip: %v", miss, ip)
}
