/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/8 12:22 上午
 * @Desc: TODO
 */

package ws_test

import (
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/network/ws"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := ws.NewClient()

	client.OnConnect(func(conn network.Conn) {
		log.Info("connection is opened")
	})
	client.OnDisconnect(func(conn network.Conn) {
		log.Info("connection is closed")
	})
	client.OnReceive(func(conn network.Conn, msg []byte, msgType int) {
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
