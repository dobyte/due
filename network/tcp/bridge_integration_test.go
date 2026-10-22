package tcp_test

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/encoding/json"
	locatememory "github.com/dobyte/due/v2/locate/memory"
	"github.com/dobyte/due/v2/packet"
	registrymemory "github.com/dobyte/due/v2/registry/memory"
)

// bridgeRoundTrip exercises the public TCP connection, Gate, real Node router,
// and Gate response path. Both transports use identical in-memory control planes.
func bridgeRoundTrip(tb testing.TB, local, listen bool) func() error {
	return bridgeClients(tb, local, listen, 256, 1)[0]
}

// Each client owns a connection and may have one request in flight. Discovery,
// connection establishment, and warmup complete before the benchmark timer starts.
func bridgeClients(tb testing.TB, local, listen bool, size, clients int) []func() error {
	tb.Helper()
	if size > 5000 {
		previous := packet.GetPacker()
		packet.SetPacker(packet.NewPacker(packet.WithBufferBytes(size + 64)))
		tb.Cleanup(func() { packet.SetPacker(previous) })
	}
	r := registrymemory.NewRegistry()
	l := locatememory.NewLocator()
	payload := bytes.Repeat([]byte("x"), size)
	n := node.NewNode(node.WithID("bridge-node"), node.WithRegistry(r), node.WithLocator(l),
		node.WithCodec(json.DefaultCodec), node.WithAddr("127.0.0.1:0"),
		node.WithLocalBridge(local), node.WithLinkerListen(listen))
	n.Proxy().AddRouteHandler(1, func(ctx node.Context) { _ = ctx.Response(payload) })
	s := tcp.NewServer(tcp.WithServerAddr("127.0.0.1:0"), tcp.WithServerHeartbeatInterval(0),
		tcp.WithServerCloseTimeout(time.Second))
	g := gate.NewGate(gate.WithID("bridge-gate"), gate.WithRegistry(r), gate.WithLocator(l),
		gate.WithServer(s), gate.WithAddr("127.0.0.1:0"), gate.WithLocalBridge(local), gate.WithLinkerListen(listen))
	n.Init()
	g.Init()
	n.Start()
	g.Start()
	conns := make([]net.Conn, 0, clients)
	tb.Cleanup(func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
		g.Close()
		g.Destroy()
		n.Close()
		n.Destroy()
		_ = l.Close()
		_ = r.Close()
	})
	roundTrips := make([]func() error, 0, clients)
	for range clients {
		conn, err := net.DialTimeout("tcp", s.Addr(), time.Second)
		if err != nil {
			tb.Fatal(err)
		}
		conns = append(conns, conn)
		roundTrip := func() error {
			if err := conn.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				return err
			}
			out, err := packet.PackMessage(1, 7, payload)
			if err != nil {
				return err
			}
			_, err = conn.Write(out.Bytes())
			out.Release()
			if err != nil {
				return err
			}
			_, _, in, err := packet.Read(conn)
			if err != nil {
				return err
			}
			route, seq, message, err := packet.UnpackMessage(in)
			if err != nil {
				in.Release()
				return err
			}
			valid := route == 1 && seq == 7 && bytes.Equal(message.Bytes(), payload)
			message.Release()
			if !valid {
				return fmt.Errorf("unexpected response route=%d seq=%d", route, seq)
			}
			return nil
		}
		// Discovery snapshots are asynchronous. Warm up before measuring or asserting.
		deadline := time.Now().Add(3 * time.Second)
		for {
			if err = roundTrip(); err == nil {
				break
			}
			if time.Now().After(deadline) {
				tb.Fatal(err)
			}
		}
		roundTrips = append(roundTrips, roundTrip)
	}
	return roundTrips
}

func TestBridgeConcurrentRoundTrip(t *testing.T) {
	for _, local := range []bool{true, false} {
		for _, size := range []int{1024, 16 << 10} {
			t.Run(fmt.Sprintf("local=%v/bytes=%d", local, size), func(t *testing.T) {
				roundTrips := bridgeClients(t, local, true, size, 8)
				var wg sync.WaitGroup
				for _, roundTrip := range roundTrips {
					wg.Go(func() {
						for range 20 {
							if err := roundTrip(); err != nil {
								t.Error(err)
								return
							}
						}
					})
				}
				wg.Wait()
			})
		}
	}
}

// ns/op measures aggregate throughput with multiple clients, not request latency.
func BenchmarkBridgeLoad(b *testing.B) {
	for _, local := range []bool{true, false} {
		transport := "drpc"
		if local {
			transport = "local"
		}
		for _, size := range []int{64, 256, 1024, 16 << 10} {
			for _, clients := range []int{1, 8, 32} {
				b.Run(fmt.Sprintf("%s/bytes=%d/clients=%d", transport, size, clients), func(b *testing.B) {
					roundTrips := bridgeClients(b, local, true, size, clients)
					var wg sync.WaitGroup
					b.ReportAllocs()
					b.SetBytes(int64(size))
					b.ResetTimer()
					for worker, roundTrip := range roundTrips {
						wg.Go(func() {
							for i := worker; i < b.N; i += clients {
								if err := roundTrip(); err != nil {
									b.Error(err)
									return
								}
							}
						})
					}
					wg.Wait()
					b.StopTimer()
				})
			}
		}
	}
}

func TestBridgeRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name          string
		local, listen bool
	}{
		{name: "local-only", local: true},
		{name: "auto", local: true, listen: true},
		{name: "drpc", listen: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roundTrip := bridgeRoundTrip(t, tc.local, tc.listen)
			for i := 0; i < 20; i++ {
				if err := roundTrip(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBridgeRoundTrip(b *testing.B) {
	for _, local := range []bool{true, false} {
		name := "drpc"
		if local {
			name = "local"
		}
		b.Run(name, func(b *testing.B) {
			roundTrip := bridgeRoundTrip(b, local, true)
			samples := make([]int64, 0, min(b.N, 100000))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				if err := roundTrip(); err != nil {
					b.Fatal(err)
				}
				if len(samples) < cap(samples) {
					samples = append(samples, time.Since(start).Nanoseconds())
				}
			}
			b.StopTimer()
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			if len(samples) > 0 {
				b.ReportMetric(float64(samples[(len(samples)-1)*50/100])/1000, "p50-us")
				b.ReportMetric(float64(samples[(len(samples)-1)*95/100])/1000, "p95-us")
				b.ReportMetric(float64(samples[(len(samples)-1)*99/100])/1000, "p99-us")
			}
		})
	}
}
