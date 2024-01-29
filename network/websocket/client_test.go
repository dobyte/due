/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/8 12:22 上午
 * @Desc: TODO
 */

package websocket_test

import (
	"github.com/dobyte/due/network/websocket/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"sync"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := websocket.NewClient()

	client.OnConnect(func(conn network.Conn) {
		log.Info("connection is opened")
	})
	client.OnDisconnect(func(conn network.Conn) {
		log.Info("connection is closed")
	})
	client.OnReceive(func(conn network.Conn, msg []byte) {
		log.Infof("receive msg from server, msg: %s", string(msg))
	})

	conn, err := client.Dial()
	if err != nil {
		log.Fatalf("dial failed: %v", err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer conn.Close()

	times := 0

	for {
		select {
		case <-ticker.C:
			if err = conn.Push([]byte("hello server~~")); err != nil {
				log.Errorf("push message failed: %v", err)
				return
			}

			times++

			if times >= 5 {
				return
			}
		}
	}
}

func BenchmarkClient(b *testing.B) {
	wg := sync.WaitGroup{}
	client := websocket.NewClient()
	client.OnReceive(func(conn network.Conn, msg []byte) {
		//_, err := packet.UnpackMessage(msg)
		//if err != nil {
		//	b.Error(err)
		//	return
		//}
		wg.Done()
	})

	msg, err := packet.PackMessage(&packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello server~~"),
	})
	if err != nil {
		b.Fatal(err)
	}

	conn, err := client.Dial()
	if err != nil {
		b.Fatal(err)
	}

	defer conn.Close()

	for i := 0; i < b.N; i++ {
		if err = conn.Push(msg); err != nil {
			b.Error(err)
			return
		} else {
			wg.Add(1)
		}
	}

	wg.Wait()
}
