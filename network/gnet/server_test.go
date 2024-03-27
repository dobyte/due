/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/11 11:31 上午
 * @Desc: TODO
 */

package gnet_test

import (
	"fmt"
	"github.com/dobyte/due/network/gnet/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"net/http"
	_ "net/http/pprof"
	"testing"
)

func TestServer(t *testing.T) {
	server := gnet.NewServer(gnet.WithServerHeartbeatMechanism(gnet.TickHeartbeat))
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
		message, err := packet.UnpackMessage(msg)
		if err != nil {
			t.Error(err)
			return
		}

		t.Logf("receive msg from client, connection id: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))

		msg, err = packet.PackMessage(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: []byte("I'm fine~~"),
		})
		if err != nil {
			t.Error(err)
			return
		}

		if err = conn.Push(msg); err != nil {
			t.Error(err)
		}
	})

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	go func() {
		err := http.ListenAndServe(":8089", nil)
		if err != nil {
			log.Errorf("pprof server start failed: %v", err)
		}
	}()

	select {}
}

func TestServer_Benchmark(t *testing.T) {
	server := gnet.NewServer()
	server.OnStart(func() {
		fmt.Println("server is started")
	})
	server.OnReceive(func(conn network.Conn, msg []byte) {
		_, err := packet.UnpackMessage(msg)
		if err != nil {
			t.Error(err)
			return
		}

		msg, err = packet.PackMessage(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: []byte("I'm fine~~"),
		})
		if err != nil {
			t.Error(err)
			return
		}

		if err = conn.Send(msg); err != nil {
			t.Error(err)
		}
	})

	if err := server.Start(); err != nil {
		t.Fatal(err)
	}

	go func() {
		err := http.ListenAndServe(":8089", nil)
		if err != nil {
			fmt.Printf("pprof server start failed: %v\n", err)
		}
	}()

	select {}
}
