/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/8 12:22 上午
 * @Desc: TODO
 */

package ws_test

import (
	"testing"
	"time"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network"
	"github.com/dobyte/due/network/ws"
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
	for {
		select {
		case <-ticker.C:
			if err = conn.Push([]byte("hello server~~")); err != nil {
				log.Errorf("push message failed: %v", err)
				return
			}
			goto OVER
		}
	}
OVER:

	select {}
}
