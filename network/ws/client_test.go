package ws_test

import (
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func TestClientServerEcho(t *testing.T) {
	addr := reserveAddr(t)

	server := ws.NewServer(ws.WithServerAddr(addr), ws.WithServerHeartbeatInterval(0))
	server.OnReceive(func(conn network.Conn, data []byte) {
		if err := conn.Push(data); err != nil {
			t.Errorf("push echo message failed: %v", err)
		}
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Fatalf("stop server failed: %v", err)
		}
	}()

	received := make(chan []byte, 1)
	client := ws.NewClient(
		ws.WithClientUrl("ws://"+addr),
		ws.WithClientHeartbeatInterval(0),
	)
	client.OnReceive(func(conn network.Conn, data []byte) {
		received <- data
	})

	conn, err := client.Dial()
	if err != nil {
		t.Fatalf("dial server failed: %v", err)
	}
	defer conn.Close(true)

	want, err := packet.PackMessage(&packet.Message{
		Seq:    1,
		Route:  2,
		Buffer: []byte("hello websocket"),
	})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	if err = conn.Push(want); err != nil {
		t.Fatalf("push message failed: %v", err)
	}

	select {
	case got := <-received:
		message, err := packet.UnpackMessage(got)
		if err != nil {
			t.Fatalf("unpack echo message failed: %v", err)
		}
		if message.Seq != 1 || message.Route != 2 || string(message.Buffer) != "hello websocket" {
			t.Fatalf("unexpected echo message: seq=%d route=%d body=%q", message.Seq, message.Route, string(message.Buffer))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for echo message")
	}
}

func reserveAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve addr failed: %v", err)
	}
	defer ln.Close()

	return ln.Addr().String()
}
