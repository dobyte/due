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
	kcpgo "github.com/xtaci/kcp-go/v5"
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

// TestServerProxyProtocol verifies that the PROXY protocol header is consumed before the first
// frame and that the real client address is reported.
func TestServerProxyProtocol(t *testing.T) {
	server := kcp.NewServer(
		kcp.WithServerListenAddr("127.0.0.1:0"),
		kcp.WithServerHeartbeatInterval(0),
		kcp.WithServerEnableProxyProtocol(true),
	)

	type receivedMessage struct {
		ip   string
		data string
	}

	received := make(chan receivedMessage, 1)
	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()

		_, _, data, err := packet.UnpackMessage(buf)
		if err != nil {
			t.Errorf("unpack message failed: %v", err)
			return
		}

		ip, err := conn.RemoteIP()
		if err != nil {
			t.Errorf("get remote ip failed: %v", err)
		}

		received <- receivedMessage{ip: ip, data: string(data.Bytes())}
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer func() { _ = server.Stop() }()

	client, err := kcpgo.DialWithOptions(server.Addr(), nil, 0, 0)
	if err != nil {
		t.Fatalf("dial server failed: %v", err)
	}
	defer client.Close()

	msg, err := packet.PackMessage(1, 1, []byte("ping"))
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	defer msg.Release()

	// The PROXY protocol header must precede the first frame in the same stream.
	header := []byte("PROXY TCP4 1.2.3.4 5.6.7.8 1234 5678\r\n")

	if _, err = client.Write(append(header, msg.Bytes()...)); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	select {
	case got := <-received:
		if got.data != "ping" {
			t.Errorf("unexpected payload: %q", got.data)
		}
		if got.ip != "1.2.3.4" {
			t.Errorf("unexpected remote ip: %q", got.ip)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("message was not received")
	}
}
