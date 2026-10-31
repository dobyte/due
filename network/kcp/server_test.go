package kcp_test

import (
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/network/kcp/v2"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func TestServer_Simple(t *testing.T) {
	server := kcp.NewServer(kcp.WithServerHeartbeatInterval(0))

	server.OnStart(func() {
		log.Info("server is started")
	})

	server.OnStop(func() {
		log.Info("server is stopped")
	})

	server.OnConnect(func(conn network.Conn) {
		log.Infof("connection is opened, connection id: %d", conn.ID())
	})

	server.OnDisconnect(func(conn network.Conn) {
		log.Infof("connection is closed, connection id: %d", conn.ID())
	})

	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()

		route, seq, data, err := packet.UnpackMessage(buf)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		log.Infof("receive message from client, cid: %d, seq: %d, route: %d, msg: %s", conn.ID(), seq, route, string(data.Bytes()))

		msg, err := packet.PackMessage(1, 1, []byte("I'm fine~~"))
		if err != nil {
			log.Errorf("pack message failed: %v", err)
			return
		}

		if err = conn.Push(msg); err != nil {
			log.Errorf("push message failed: %v", err)
		}
	})

	if err := server.Start(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	select {}
}

func TestServer_Benchmark(t *testing.T) {
	server := kcp.NewServer(
		kcp.WithServerHeartbeatInterval(0),
	)

	server.OnStart(func() {
		log.Info("server is started")
	})

	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()

		route, seq, data, err := packet.UnpackMessage(buf)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		msg, err := packet.PackMessage(route, seq, data.Bytes())
		if err != nil {
			log.Errorf("pack message failed: %v", err)
			return
		}

		if err = conn.Push(msg); err != nil {
			log.Errorf("push message failed: %v", err)
			return
		}
	})

	if err := server.Start(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	go func() {
		err := http.ListenAndServe(":8089", nil)
		if err != nil {
			log.Errorf("pprof server start failed: %v", err)
		}
	}()

	select {}
}

// TestServerConnectHookBeforeReceive verifies that the connect hook completes before any frame is
// handled.
func TestServerConnectHookBeforeReceive(t *testing.T) {
	server := kcp.NewServer(kcp.WithServerListenAddr("127.0.0.1:0"))

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

	client := kcp.NewClient()
	conn, err := client.Dial(server.Addr())
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
