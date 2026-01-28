package kcp_test

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/network/kcp/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xrand"
)

func TestClient_Simple(t *testing.T) {
	client := kcp.NewClient(kcp.WithClientHeartbeatInterval(0))

	client.OnConnect(func(conn network.Conn) {
		log.Info("connection is opened")
	})

	client.OnDisconnect(func(conn network.Conn) {
		log.Info("connection is closed")
	})

	client.OnReceive(func(conn network.Conn, data []byte) {
		message, err := packet.UnpackMessage(data)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		log.Infof("receive msg from server, cid: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))
	})

	conn, err := client.Dial()
	if err != nil {
		log.Fatalf("client dial failed: %v", err)
	}
	defer conn.Close()

	time.Sleep(10 * time.Second)

	counter := 0

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			msg, err := packet.PackMessage(&packet.Message{
				Seq:    1,
				Route:  1,
				Buffer: []byte("hello server~~"),
			})
			if err != nil {
				log.Errorf("pack message failed: %v", err)
				continue
			}

			if err = conn.Push(msg); err != nil {
				log.Errorf("push message failed: %v", err)
				return
			}

			counter++

			if counter%5 == 0 {
				conn.Close()
			}

			if counter >= 200 {
				return
			}
		}
	}
}

func TestClient_Benchmark(t *testing.T) {
	samples := []struct {
		c    int // 并发数
		n    int // 请求数
		size int // 数据包大小
	}{
		{
			c:    50,
			n:    1000000,
			size: 1024,
		},
		{
			c:    100,
			n:    1000000,
			size: 1024,
		},
		{
			c:    200,
			n:    1000000,
			size: 1024,
		},
		{
			c:    300,
			n:    1000000,
			size: 1024,
		},
		{
			c:    400,
			n:    1000000,
			size: 1024,
		},
		{
			c:    500,
			n:    1000000,
			size: 1024,
		},
		{
			c:    1000,
			n:    1000000,
			size: 2 * 1024,
		},
	}

	go func() {
		err := http.ListenAndServe(":8090", nil)
		if err != nil {
			log.Errorf("pprof server start failed: %v", err)
		}
	}()

	for _, sample := range samples {
		doPressureTest(sample.c, sample.n, sample.size)
	}
}

// 执行压力测试
func doPressureTest(c int, n int, size int) {
	var (
		wg        sync.WaitGroup
		totalSent int64
		totalRecv int64
	)

	client := kcp.NewClient(kcp.WithClientHeartbeatInterval(0))

	client.OnReceive(func(conn network.Conn, data []byte) {
		atomic.AddInt64(&totalRecv, 1)

		wg.Done()
	})

	buffer := []byte(xrand.Letters(size))

	chMsg := make(chan struct{}, n)

	for i := 0; i < c; i++ {
		conn, err := client.Dial()
		if err != nil {
			log.Errorf("client dial failed: %v", err)
			i--
			continue
		}

		go func(conn network.Conn) {
			defer conn.Close(true)

			for {
				select {
				case _, ok := <-chMsg:
					if !ok {
						return
					}

					msg, err := packet.PackMessage(&packet.Message{
						Seq:    1,
						Route:  1,
						Buffer: buffer,
					})
					if err != nil {
						log.Errorf("pack message failed: %v", err)
						return
					}

					if err = conn.Push(msg); err != nil {
						log.Errorf("push message failed: %v", err)
						return
					}

					atomic.AddInt64(&totalSent, 1)
				}
			}
		}(conn)
	}

	wg.Add(n)

	startTime := time.Now().UnixNano()

	for i := 0; i < n; i++ {
		chMsg <- struct{}{}
	}

	wg.Wait()

	close(chMsg)

	totalTime := float64(time.Now().UnixNano()-startTime) / float64(time.Second)

	fmt.Printf("server               : %s\n", client.Protocol())
	fmt.Printf("concurrency          : %d\n", c)
	fmt.Printf("latency              : %fs\n", totalTime)
	fmt.Printf("data size            : %s\n", convBytes(size))
	fmt.Printf("sent requests        : %d\n", totalSent)
	fmt.Printf("received requests    : %d\n", totalRecv)
	fmt.Printf("throughput (TPS)     : %d\n", int64(float64(totalRecv)/totalTime))
	fmt.Printf("--------------------------------\n")
}

func convBytes(bytes int) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%.2fB", float64(bytes))
	case bytes < MB:
		return fmt.Sprintf("%.2fKB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.2fMB", float64(bytes)/MB)
	case bytes < TB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/GB)
	default:
		return fmt.Sprintf("%.2fTB", float64(bytes)/TB)
	}
}
