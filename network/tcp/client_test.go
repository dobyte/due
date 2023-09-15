package tcp_test

import (
	"testing"
	"time"

	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/network/tcp"
)

func TestNewClient(t *testing.T) {
	client := tcp.NewClient()

	client.OnConnect(func(conn network.Conn) {
		t.Log("connection is opened")
	})
	client.OnDisconnect(func(conn network.Conn) {
		t.Log("connection is closed")
	})
	client.OnReceive(func(conn network.Conn, msg []byte, msgType int) {
		t.Logf("receive msg from server, msg: %s", string(msg))
	})

	conn, err := client.Dial()
	if err != nil {
		t.Fatal(err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer conn.Close()

	times := 0

	for {
		select {
		case <-ticker.C:
			if err = conn.Push([]byte("hello server~~")); err != nil {
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
