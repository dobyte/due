package tcp_test

import (
	"fmt"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_Dial(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 1; i++ {
		wg.Add(1)

		go func() {
			client := tcp.NewClient()
			client.OnConnect(func(conn network.Conn) {
				t.Log("connection is opened")
			})
			client.OnDisconnect(func(conn network.Conn) {
				t.Log("connection is closed")
			})
			client.OnReceive(func(conn network.Conn, msg []byte) {
				fmt.Println(msg)
				message, err := packet.UnpackMessage(msg)
				if err != nil {
					t.Error(err)
					return
				}

				t.Logf("receive msg from server, connection id: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))
			})

			defer wg.Done()

			conn, err := client.Dial()
			if err != nil {
				t.Fatal(err)
			}

			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			defer conn.Close()

			times := 0
			msg, _ := packet.PackMessage(&packet.Message{
				Seq:    1,
				Route:  1,
				Buffer: []byte("hello server~~"),
			})

			for {
				select {
				case <-ticker.C:
					fmt.Println(222)
					if err = conn.Send(msg); err != nil {
						t.Error(err)
						return
					}

					times++

					if times >= 200 {
						return
					}
				}
			}
		}()
	}

	wg.Wait()
}

func Test_Benchmark(t *testing.T) {
	// 并发数
	concurrency := 500
	// 消息量
	total := 10000000
	// 总共发送的消息条数
	totalSent := int64(0)
	// 总共接收的消息条数
	totalRecv := int64(0)

	wg := sync.WaitGroup{}
	client := tcp.NewClient()
	client.OnReceive(func(conn network.Conn, msg []byte) {

		atomic.AddInt64(&totalRecv, 1)

		//fmt.Println("recv num: ", atomic.AddInt64(&totalRecv, 1))

		wg.Done()
	})

	wg.Add(total)

	chMsg := make(chan struct{}, total)

	// 准备连接
	conns := make([]network.Conn, 0, concurrency)
	for i := 0; i < concurrency; i++ {
		conn, err := client.Dial()
		if err != nil {
			fmt.Println("connect failed", i, err)
			i--
			continue
		}

		conns = append(conns, conn)
		time.Sleep(time.Millisecond)
	}

	// 发送消息
	for _, conn := range conns {
		go func(conn network.Conn) {
			defer conn.Close(true)

			for {
				select {
				case _, ok := <-chMsg:
					if !ok {
						return
					}

					// 准备消息
					msg, err := packet.PackMessage(&packet.Message{
						Seq:    1,
						Route:  1,
						Buffer: []byte("hello server~~"),
					})
					if err != nil {
						t.Error(err)
						return
					}

					if err = conn.Send(msg); err != nil {
						t.Error(err)
						return
					}

					atomic.AddInt64(&totalSent, 1)

					//fmt.Println("sent num: ", atomic.AddInt64(&totalSent, 1))
				}
			}
		}(conn)
	}

	startTime := time.Now().UnixNano()

	for i := 0; i < total; i++ {
		chMsg <- struct{}{}
	}

	wg.Wait()
	close(chMsg)

	totalTime := float64(time.Now().UnixNano()-startTime) / float64(time.Second)

	fmt.Printf("server               : %s\n", "tcp")
	fmt.Printf("concurrency          : %d\n", concurrency)
	fmt.Printf("latency              : %fs\n", totalTime)
	fmt.Printf("sent     requests    : %d\n", totalSent)
	fmt.Printf("received requests    : %d\n", totalRecv)
	fmt.Printf("throughput  (TPS)    : %d\n", int64(float64(totalRecv)/totalTime))
}
