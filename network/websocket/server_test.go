/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/29 10:59 上午
 * @Desc: TODO
 */

package websocket_test

import (
	"github.com/dobyte/due/network/websocket/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"net/http"
	"testing"
)

func TestServer(t *testing.T) {
	server := websocket.NewServer()
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

	select {}
}
