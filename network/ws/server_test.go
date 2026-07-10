package ws_test

import (
	"net"
	"testing"

	"github.com/dobyte/due/network/ws/v2"
)

func TestServerStartFailureRollsBackState(t *testing.T) {
	addr := reserveAddr(t)
	server := ws.NewServer(ws.WithServerAddr(addr), ws.WithServerHeartbeatInterval(0))

	if err := server.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("stop server failed: %v", err)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("occupy addr failed: %v", err)
	}

	if err = server.Start(); err == nil {
		ln.Close()
		defer server.Stop()
		t.Fatal("expected start to fail while addr is occupied")
	}
	if err = ln.Close(); err != nil {
		t.Fatalf("release occupied addr failed: %v", err)
	}

	if err = server.Start(); err != nil {
		t.Fatalf("restart after failed start should succeed: %v", err)
	}
	if err = server.Stop(); err != nil {
		t.Fatalf("stop restarted server failed: %v", err)
	}
}
