package drpc_test

import (
	_ "net/http/pprof"
	"testing"

	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/log"
)

func TestServer_Simple(t *testing.T) {
	server, err := drpc.NewServer(&drpc.ServerOptions{
		Addr: "127.0.0.1:8080",
	})
	if err != nil {
		log.Fatalf("create server failed: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	select {}
}

// func TestServer_Benchmark(t *testing.T) {
// 	server := tcp.NewServer(
// 		tcp.WithServerHeartbeatInterval(0),
// 	)

// 	server.OnStart(func() {
// 		log.Info("server is started")
// 	})

// 	server.OnReceive(func(conn network.Conn, data []byte) {
// 		message, err := packet.UnpackMessage(data)
// 		if err != nil {
// 			log.Errorf("unpack message failed: %v", err)
// 			return
// 		}

// 		msg, err := packet.PackMessage(&packet.Message{
// 			Seq:    message.Seq,
// 			Route:  message.Route,
// 			Buffer: message.Buffer,
// 		})
// 		if err != nil {
// 			log.Errorf("pack message failed: %v", err)
// 			return
// 		}

// 		if err = conn.Push(msg); err != nil {
// 			log.Errorf("push message failed: %v", err)
// 			return
// 		}
// 	})

// 	if err := server.Start(); err != nil {
// 		log.Fatalf("start server failed: %v", err)
// 	}

// 	go func() {
// 		err := http.ListenAndServe(":8089", nil)
// 		if err != nil {
// 			log.Errorf("pprof server start failed: %v", err)
// 		}
// 	}()

// 	select {}
// }
