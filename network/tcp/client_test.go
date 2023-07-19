package tcp_test

import (
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := tcp.NewClient()

	client.OnConnect(func(conn network.Conn) {
		t.Log("connection is opened")
	})
	client.OnDisconnect(func(conn network.Conn) {
		t.Log("connection is closed")
	})
	client.OnReceive(func(conn network.Conn, msg []byte) {
		message, err := packet.Unpack(msg)
		if err != nil {
			t.Error(err)
			return
		}

		t.Logf("receive msg from server, connection id: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))
	})

	conn, err := client.Dial()
	if err != nil {
		t.Fatal(err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer conn.Close()

	times := 0
	msg, _ := packet.Pack(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello server~~"),
	})

	for {
		select {
		case <-ticker.C:
			if err = conn.Push(msg); err != nil {
				t.Error(err)
				return
			}

			times++

			if times >= 5 {
				return
			}
		}
	}
}
