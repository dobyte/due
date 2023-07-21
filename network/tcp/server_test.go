/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/11 11:31 上午
 * @Desc: TODO
 */

package tcp_test

import (
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"testing"
)

func TestServer(t *testing.T) {
	server := tcp.NewServer()
	server.OnStart(func() {
		t.Logf("server is started")
	})
	server.OnConnect(func(conn network.Conn) {
		t.Logf("connection is opened, connection id: %d", conn.ID())
	})
	server.OnDisconnect(func(conn network.Conn) {
		t.Logf("connection is closed, connection id: %d", conn.ID())
	})
	server.OnReceive(func(conn network.Conn, msg []byte) {
		//message, err := packet.Unpack(msg)
		//if err != nil {
		//	t.Error(err)
		//	return
		//}
		//
		//t.Logf("receive msg from client, connection id: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))

		msg, err := packet.Pack(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: []byte("I'm fine~~"),
		})
		if err != nil {
			t.Error(err)
			return
		}

		go func() {
			if err = conn.Push(msg); err != nil {
				t.Error(err)
			}
		}()

		go func() {
			conn.Close(true)
		}()
	})

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	select {}
}
