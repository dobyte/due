package node_test

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/log"
	"testing"
)

func TestServer(t *testing.T) {
	server, err := node.NewServer(":49898", &provider{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("server listen on: %s", server.Addr())

	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
}

type provider struct {
}

// Trigger 触发事件
func (p *provider) Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error {
	return nil
}

// Deliver 投递消息
func (p *provider) Deliver(ctx context.Context, gid, nid string, cid, uid int64, message []byte) error {
	log.Infof("gid: %s, nid: %s, cid: %d, uid: %d message: %s", gid, nid, cid, uid, string(message))
	return nil
}
