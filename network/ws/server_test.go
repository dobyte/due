/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/29 10:59 上午
 * @Desc: TODO
 */

package ws_test

import (
	"fmt"
	"github.com/dobyte/due/network/ws"
	"net/http"
	"testing"

	"github.com/dobyte/due/network"
)

func TestServer(t *testing.T) {
	server := ws.NewServer(
		ws.WithAddr(":8088"),
		ws.WithMaxConnNum(5),
		ws.WithCheckOrigin(func(r *http.Request) bool { return true }),
	)

	server.OnStart(func(s network.Server) {
		fmt.Println("server is started")
	})
	server.OnConnect(func(s network.Server, conn network.Conn) {
		fmt.Printf("conn is opened, conn id: %d\n", conn.ID())
	})
	server.OnDisconnect(func(s network.Server, conn network.Conn) {
		fmt.Printf("conn is closed, conn id: %d\n", conn.ID())
	})
	server.OnReceive(func(s network.Server, conn network.Conn, msg []byte, msgType int) {
		fmt.Printf("receive msg from conn, conn id: %d, msg: %s\n", conn.ID(), string(msg))
		_ = conn.Close()
		if err := conn.Send([]byte("hello world")); err != nil {
			fmt.Println(err)
		}
	})

	_ = server.Start()

	select {}
}
