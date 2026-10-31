package tcp_test

import (
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func TestServer_Simple(t *testing.T) {
	server := tcp.NewServer()

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
	server := tcp.NewServer(
		tcp.WithServerHeartbeatInterval(0),
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

type chWrite struct {
	msg []byte
	// typ int8
}

func TestServer_ChWrite(t *testing.T) {
	v := unsafe.Sizeof(&chWrite{})

	t.Logf("chWrite size: %d", v)
}

// TestServerConnectHookBeforeReceive verifies that the connect hook completes before any frame is
// handled.
func TestServerConnectHookBeforeReceive(t *testing.T) {
	server := tcp.NewServer(tcp.WithServerAddr("127.0.0.1:0"))

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

	client := tcp.NewClient()
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

// TestServerDisconnectAfterConnectHook verifies that the disconnect hook runs only after the
// connect hook returns, even when the connection is closed concurrently while the connect hook is
// still running.
func TestServerDisconnectAfterConnectHook(t *testing.T) {
	server := tcp.NewServer(tcp.WithServerAddr("127.0.0.1:0"))

	var (
		order   atomic.Int32
		entered = make(chan struct{})
		release = make(chan struct{})
	)

	server.OnConnect(func(conn network.Conn) {
		close(entered)
		<-release
		order.Store(1)
	})

	disconnected := make(chan struct{}, 1)
	server.OnDisconnect(func(conn network.Conn) {
		if order.Load() != 1 {
			t.Error("disconnect ran before the connect hook returned")
		}

		disconnected <- struct{}{}
	})

	if err := server.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}

	clientSawClose := make(chan struct{}, 1)
	client := tcp.NewClient()
	client.OnDisconnect(func(conn network.Conn) { clientSawClose <- struct{}{} })

	conn, err := client.Dial(server.Addr())
	if err != nil {
		t.Fatalf("dial server failed: %v", err)
	}
	defer conn.Close(true)

	<-entered

	stopped := make(chan struct{})
	go func() {
		_ = server.Stop()
		close(stopped)
	}()

	// The server closes the underlying connection before waiting for the read goroutine, so the
	// client observes the close while the server is still blocked on the connect hook.
	select {
	case <-clientSawClose:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not close the connection")
	}

	close(release)

	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("disconnect hook was not invoked")
	}

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("server stop did not finish")
	}
}
