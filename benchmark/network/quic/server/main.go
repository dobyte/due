package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/dobyte/due/network/quic/v2"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func main() {
	server := quic.NewServer(
		quic.WithServerHeartbeatInterval(0),
		quic.WithServerCredentials("../certs/cert.pem", "../certs/key.pem"),
	)

	server.OnStart(func() {
		log.Info("server is started")
	})

	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()

		route, seq, message, err := packet.UnpackMessage(buf)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		msg, err := packet.PackMessage(&packet.Message{
			Seq:    seq,
			Route:  route,
			Buffer: message.Bytes(),
		})
		if err != nil {
			log.Errorf("pack message failed: %v", err)
			return
		}

		if err = conn.Push(msg); err != nil {
			msg.Release()
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
