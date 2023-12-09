package kcp_test

import (
	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/network/kcp"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := kcp.NewClient(
		kcp.WithClientDialAddr("127.0.0.1:3554"),
	)

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
	for {
		select {
		case <-ticker.C:
			if err = conn.Push([]byte("hello kcp server~~")); err != nil {
				t.Error(err)
				return
			}

			//return
		}
	}
}
