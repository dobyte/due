package main

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/gorilla/websocket"
)

func TestConnectionRate(t *testing.T) {
	for _, total := range []int{1, 100, 1001, 100003} {
		for _, conns := range []int{1, 50, 1000} {
			sum := 0
			for i := range conns {
				sum += connectionRate(total, conns, i)
			}
			if sum != total {
				t.Fatalf("total=%d conns=%d allocated=%d", total, conns, sum)
			}
		}
	}
}

func TestDefaultWindows(t *testing.T) {
	for _, tc := range []struct {
		cfg  config
		want int
	}{
		{config{mode: modeThroughput}, 32},
		{config{mode: modeLatency}, 1},
		{config{mode: modeRate}, 32},
		{config{mode: modeThroughput, unbounded: true}, 0},
		{config{mode: modeThroughput, window: 8}, 8},
	} {
		if got := (&runner{cfg: tc.cfg}).window(); got != tc.want {
			t.Fatalf("config=%+v window=%d want=%d", tc.cfg, got, tc.want)
		}
	}
}

func TestWeightedLatency(t *testing.T) {
	rd := &round{track: true, states: map[*client.Conn]*connState{
		new(client.Conn): {seen: 9900, samples: []float64{1, 1}},
		new(client.Conn): {seen: 100, samples: []float64{100, 100}},
	}}
	samples, seen := rd.mergeLatencies()
	if seen != 10000 || weightedPercentile(samples, .95) != 1 || weightedPercentile(samples, .999) != 100 {
		t.Fatalf("incorrect weighted distribution: %v, seen=%d", samples, seen)
	}
	_, _, avg, _ := weightedStats(samples)
	if math.Abs(avg-1.99) > 1e-9 {
		t.Fatalf("average=%v", avg)
	}
}

func TestWindowStatisticsExcludeDrain(t *testing.T) {
	res := result{duration: 2 * time.Second, windowRecv: 100, recv: 150, sent: 150, pushErrs: 1, drainDuration: time.Second}
	if res.tps() != 50 {
		t.Fatalf("TPS includes drained replies: %d", res.tps())
	}
	if res.successRate() >= 100 {
		t.Fatal("failed push hidden by success rate")
	}
}

func TestSenderCancellation(t *testing.T) {
	for _, rate := range []int{-1, 0} {
		ctx, cancel := context.WithCancel(context.Background())
		rd := &round{resume: make(chan struct{})}
		rd.phase.Store(phaseWarmup)
		st := &connState{window: make(chan struct{}, 1)}
		st.window <- struct{}{}
		done := make(chan struct{})
		go func() { (&runner{}).sendLoop(ctx, rd, st, 0, rate); close(done) }()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("sender ignored cancellation")
		}
	}
}

func TestIdleSenderFence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rd := &round{resume: make(chan struct{})}
	rd.phase.Store(phaseFence)
	done := make(chan struct{})
	go func() { (&runner{}).sendLoop(ctx, rd, &connState{}, 0, 0); close(done) }()
	if !waitCond(time.Second, func() bool { return rd.parked.Load() == 1 }) {
		t.Fatal("idle connection did not park")
	}
	rd.phase.Store(phaseMeasure)
	close(rd.resume)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle connection did not stop")
	}
}

// Echoing the wire frames exercises real WebSocket framing and client callbacks without Redis/Nacos.
func TestRoundWS(t *testing.T) {
	url, _ := newWSPeer(t, false)
	component := client.NewClient(client.WithCodec(json.DefaultCodec), client.WithClient(ws.NewClient(
		ws.WithClientWriteTimeout(time.Second), ws.WithClientCloseTimeout(time.Second), ws.WithClientHeartbeatInterval(0),
	)))
	r := &runner{proxy: component.Proxy()}
	component.Proxy().AddRouteHandler(greet, r.handleReply)
	component.Init()
	component.Start()
	t.Cleanup(func() { component.Close(); component.Destroy() })
	for _, mode := range []string{modeThroughput, modeLatency, modeRate} {
		t.Run(mode, func(t *testing.T) {
			r.cfg = config{mode: mode, addr: url, duration: 100 * time.Millisecond, warmup: 20 * time.Millisecond, requests: 20, rate: 1}
			res, err := r.runRound(2, 64, 1)
			if err != nil {
				t.Fatal(err)
			}
			if res.failure != "" || !res.drained || res.sent != res.recv || res.recv == 0 {
				t.Fatalf("round failed: failure=%q drained=%v sent=%d recv=%d", res.failure, res.drained, res.sent, res.recv)
			}
			if mode != modeThroughput && res.samples != res.recv {
				t.Fatalf("samples=%d replies=%d", res.samples, res.recv)
			}
		})
	}
}

func TestBlockedWSShutdown(t *testing.T) {
	url, accepted := newWSPeer(t, true)
	component := client.NewClient(client.WithCodec(json.DefaultCodec), client.WithClient(ws.NewClient(
		ws.WithClientWriteTimeout(100*time.Millisecond), ws.WithClientCloseTimeout(100*time.Millisecond), ws.WithClientHeartbeatInterval(0),
	)))
	component.Init()
	component.Start()
	defer func() { component.Close(); component.Destroy() }()
	conn, err := component.Proxy().Dial(client.WithDialAddr(url))
	if err != nil {
		t.Fatal(err)
	}
	peer := <-accepted
	if peer == nil {
		t.Fatal("accept failed")
	}
	defer peer.Close()
	// The peer deliberately never reads: writes must eventually encounter backpressure.
	st := &connState{conn: conn, req: greetReq{Message: strings.Repeat("x", 1024)}}
	rd := &round{origin: time.Now(), states: map[*client.Conn]*connState{conn: st}}
	rd.phase.Store(phaseWarmup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var senders sync.WaitGroup
	senders.Add(1)
	go func() { defer senders.Done(); (&runner{}).sendLoop(ctx, rd, st, 0, -1) }()
	if !waitCond(5*time.Second, func() bool { return rd.aborted.Load() }) {
		cancel()
		rd.phase.Store(phaseStop)
		_ = peer.Close()
		rd.closeConns()
		senders.Wait()
		t.Fatal("non-reading peer did not cause a bounded push failure")
	}
	start := time.Now()
	_ = (&runner{}).abortRound(rd, cancel, &senders, "test backpressure")
	if time.Since(start) > 3*time.Second {
		t.Fatal("shutdown exceeded its budget")
	}
}

// newWSPeer performs an HTTP upgrade and either echoes binary frames or stops reading.
func newWSPeer(t *testing.T, blocked bool) (string, <-chan *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	stop := make(chan struct{})
	var peers sync.WaitGroup
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peers.Add(1)
		defer peers.Done()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if blocked {
			accepted <- conn
			<-stop
			return
		}
		for {
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(kind, data); err != nil {
				return
			}
		}
	}))
	t.Cleanup(func() { close(stop); server.Close(); peers.Wait() })
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/", accepted
}
