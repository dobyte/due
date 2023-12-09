/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/11 11:31 上午
 * @Desc: TODO
 */

package kcp_test

import (
	"github.com/symsimmy/due/network/kcp"
	"testing"
	"time"

	"github.com/symsimmy/due/network"
)

func TestServer(t *testing.T) {
	server := kcp.NewServer(
		kcp.WithServerListenAddr(":3553"),
		kcp.WithServerMaxConnNum(5),
		kcp.WithServerMaxMsgLen(10),
		kcp.WithServerHeartbeatInterval(10*time.Second),
	)
	server.OnStart(func() {
		t.Logf("server is started")
	})
	server.OnConnect(func(conn network.Conn) {
		t.Logf("connection is opened, connection id: %d", conn.ID())
	})
	server.OnDisconnect(func(conn network.Conn) {
		t.Logf("connection is closed, connection id: %d", conn.ID())
	})
	server.OnReceive(func(conn network.Conn, msg []byte, msgType int) {
		t.Logf("receive msg from client, connection id: %d, msg: %s", conn.ID(), string(msg))

		if err := conn.Push([]byte("I'm fine~~")); err != nil {
			t.Error(err)
		}
	})

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	select {}
}
