package tcp_test

import (
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"sync"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		client := tcp.NewClient()

		client.OnConnect(func(conn network.Conn) {
			t.Log("connection is opened")
		})
		client.OnDisconnect(func(conn network.Conn) {
			t.Log("connection is closed")
		})
		client.OnReceive(func(conn network.Conn, msg []byte) {
			message, err := packet.Unpack(msg)
			if err != nil {
				t.Error(err)
				return
			}

			t.Logf("receive msg from server, connection id: %d, seq: %d, route: %d, msg: %s", conn.ID(), message.Seq, message.Route, string(message.Buffer))
		})

		conn, err := client.Dial()
		if err != nil {
			t.Fatal(err)
		}

		msg, _ := packet.Pack(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: []byte("hello server~~"),
		})

		conn.Send(msg)
		conn.Close(true)
		time.Sleep(time.Millisecond)
	}
}

func TestNewClient_Dial(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 400; i++ {
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
				message, err := packet.Unpack(msg)
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
			msg, _ := packet.Pack(&packet.Message{
				Seq:    1,
				Route:  1,
				Buffer: []byte("hello server~~"),
			})

			for {
				select {
				case <-ticker.C:
					if err = conn.Push(msg); err != nil {
						t.Error(err)
						return
					}

					times++

					if times >= 5 {
						return
					}
				}
			}
		}()
	}

	wg.Wait()
}

func BenchmarkClient(b *testing.B) {
	wg := sync.WaitGroup{}
	client := tcp.NewClient()
	client.OnReceive(func(conn network.Conn, msg []byte) {
		_, err := packet.Unpack(msg)
		if err != nil {
			b.Error(err)
			return
		}
		wg.Done()
	})

	msg, err := packet.Pack(&packet.Message{
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
