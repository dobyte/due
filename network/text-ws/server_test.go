/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/29 10:59 上午
 * @Desc: TODO
 */

package ws_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network/ws"

	"github.com/symsimmy/due/network"
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
	server.OnReceive(func(conn network.Conn, msg []byte, msgType int) {
		log.Infof("receive msg from client, connection id: %d, msg: %s", conn.ID(), string(msg))

		if err := conn.Push([]byte("I'm fine~~")); err != nil {
			log.Errorf("push message failed: %v", err)
		}
	})
	server.OnUpgrade(func(w http.ResponseWriter, r *http.Request) (allowed bool) {
		return true
	})

	if err := server.Start(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	time.Sleep(30 * time.Second)
}
