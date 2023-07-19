/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/29 10:59 上午
 * @Desc: TODO
 */

package ws_test

import (
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"net/http"
	"testing"
)

func TestServer(t *testing.T) {
	server := ws.NewServer()
	server.OnStart(func() {
		log.Info("server is started")
	})
	server.OnConnect(func(conn network.Conn) {
		log.Infof("connection is opened, connection id: %d", conn.ID())
	})
	server.OnDisconnect(func(conn network.Conn) {
		log.Infof("connection is closed, connection id: %d", conn.ID())
	})
	server.OnReceive(func(conn network.Conn, msg []byte) {
		message, err := packet.Unpack(msg)
		if err != nil {
			t.Error(err)
			return
		}

		t.Logf("receive msg from client, connection id: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))

		msg, err = packet.Pack(&packet.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: []byte("I'm fine~~"),
		})

		if err = conn.Push(msg); err != nil {
			log.Errorf("push message failed: %v", err)
		}
	})
	server.OnUpgrade(func(w http.ResponseWriter, r *http.Request) (allowed bool) {
		return true
	})

	if err := server.Start(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	select {}
}
