/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/29 10:59 上午
 * @Desc: TODO
 */

package ws_test

import (
	"testing"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network/ws"

	"github.com/dobyte/due/network"
)

func TestServer(t *testing.T) {
	server := ws.NewServer(
		ws.WithServerListenAddr(":3553"),
		ws.WithServerMaxConnNum(5),
	)
	server.OnStart(func() {
		log.Info("server is started")
	})
	server.OnConnect(func(conn network.Conn) {
		log.Info("connection is opened, connection id: %d", conn.ID())
	})
	server.OnDisconnect(func(conn network.Conn) {
		log.Info("connection is closed, connection id: %d", conn.ID())
	})
	server.OnReceive(func(conn network.Conn, msg []byte, msgType int) {
		log.Info("receive msg from client, connection id: %d, msg: %s", conn.ID(), string(msg))

		if err := conn.Push([]byte("I'm fine~~")); err != nil {
			log.Errorf("push message failed: %v", err)
		}
	})

	if err := server.Start(); err != nil {
		log.Fatal("start server failed: %v", err)
	}

	select {}
}
