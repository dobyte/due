package ws_test

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
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

// TestServerConnectHookBeforeReceive verifies that the connect hook completes before any frame is
// handled.
func TestServerConnectHookBeforeReceive(t *testing.T) {
	addr := reserveAddr(t)
	server := ws.NewServer(ws.WithServerAddr(addr))

	var order atomic.Int32
	server.OnConnect(func(conn network.Conn) {
		time.Sleep(100 * time.Millisecond)
		order.Store(1)
	})

	received := make(chan struct{}, 1)
	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		buf.Release()

		if order.Load() != 1 {
			t.Error("receive ran before the connect hook returned")
		}

		received <- struct{}{}
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer func() { _ = server.Stop() }()

	client := ws.NewClient(ws.WithClientUrl("ws://" + addr))
	conn, err := client.Dial()
	if err != nil {
		t.Fatalf("dial server failed: %v", err)
	}
	defer conn.Close(true)

	buf, err := packet.PackMessage(1, 1, []byte("ping"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	if err = conn.Push(buf); err != nil {
		t.Fatalf("push message failed: %v", err)
	}

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("message was not received")
	}
}
