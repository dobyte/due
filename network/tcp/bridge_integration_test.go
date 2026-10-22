package tcp_test

import (
	"bytes"
	"fmt"
	"net"
	"sort"
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
	tb.Helper()
	r := registrymemory.NewRegistry()
	l := locatememory.NewLocator()
	payload := bytes.Repeat([]byte("x"), 256)
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
	var conn net.Conn
	tb.Cleanup(func() {
		if conn != nil {
			_ = conn.Close()
		}
		g.Close()
		g.Destroy()
		n.Close()
		n.Destroy()
		_ = l.Close()
		_ = r.Close()
	})
	var err error
	conn, err = net.DialTimeout("tcp", s.Addr(), time.Second)
	if err != nil {
		tb.Fatal(err)
	}
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
	return roundTrip
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
