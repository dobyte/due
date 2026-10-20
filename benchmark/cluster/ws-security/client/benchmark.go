package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"due-benchmark/cluster/ws-security/internal/profile"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
)

const _peerKey = "ws-security-benchmark-peer"

type options struct {
	connections int
	requests    int
	size        int
	window      int
	timeout     time.Duration
}

func (o options) validate() error {
	if o.connections <= 0 || o.requests < o.connections || o.requests > math.MaxInt32 {
		return fmt.Errorf("require 0 < c <= n <= %d", math.MaxInt32)
	}
	if o.window <= 0 || o.window > o.requests {
		return fmt.Errorf("require 0 < window <= n")
	}
	if o.size < 0 || o.size > profile.MaxMessageSize {
		return fmt.Errorf("require 0 <= size <= %d", profile.MaxMessageSize)
	}
	if o.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

type response struct {
	seq int32
	at  time.Time
	err error
}

type peer struct {
	push      func(*cluster.Message) error
	replies   chan response
	overflow  chan struct{}
	latencies []time.Duration // Written only by this peer's worker.
	attempted int
	sent      int
}

func receive(message string) client.RouteHandler {
	return func(ctx *client.Context) {
		p, ok := ctx.Conn().GetAttr(_peerKey).Value().(*peer)
		if !ok {
			return
		}
		r := response{seq: ctx.Seq(), at: time.Now()}
		var res profile.Greeting
		r.err = ctx.Parse(&res)
		if r.err == nil && res.Message != message {
			r.err = fmt.Errorf("echo payload mismatch for seq %d", r.seq)
		}
		// At most window replies are expected; never block the network receive loop.
		select {
		case p.replies <- r:
		default:
			select {
			case p.overflow <- struct{}{}:
			default:
			}
		}
	}
}

func benchmark(ctx context.Context, proxy *client.Proxy, opts options, message string) error {
	conns := make([]*client.Conn, 0, opts.connections)
	peers := make([]*peer, 0, opts.connections)
	dials := make([]time.Duration, 0, opts.connections)
	setupStart := time.Now()
	var setupErr error
	for i := 0; i < opts.connections; i++ {
		if setupErr = ctx.Err(); setupErr != nil {
			break
		}
		p := &peer{
			replies:  make(chan response, min(opts.window, (opts.requests-1)/opts.connections+1)),
			overflow: make(chan struct{}, 1),
		}
		start := time.Now()
		conn, err := proxy.Dial(client.WithConnAttr(_peerKey, p))
		if err != nil {
			setupErr = fmt.Errorf("dial/handshake connection %d: %w", i+1, err)
			break
		}
		dials = append(dials, time.Since(start))
		p.push = conn.Push
		conns = append(conns, conn)
		peers = append(peers, p)
	}
	fmt.Printf("setup: ready=%d/%d elapsed=%s (sequential WS + security handshakes)\n",
		len(conns), opts.connections, time.Since(setupStart))
	printLatency("dial/handshake", dials)
	var runErr error
	if setupErr == nil {
		runCtx, cancel := context.WithTimeout(ctx, opts.timeout)
		start := time.Now()
		runErr = runPeers(runCtx, peers, opts, message)
		elapsed := time.Since(start)
		cancel()
		printResult(peers, opts, elapsed)
	}
	for _, conn := range conns {
		if err := conn.Close(); err != nil && runErr == nil {
			runErr = fmt.Errorf("close connection: %w", err)
		}
	}
	if setupErr != nil {
		return setupErr
	}
	return runErr
}

func runPeers(ctx context.Context, peers []*peer, opts options, message string) error {
	ctx, cancel := context.WithCancel(ctx)
	errors := make(chan error, 1)
	var wg sync.WaitGroup
	startSeq := 1
	for i, p := range peers {
		count := opts.requests / len(peers)
		if i < opts.requests%len(peers) {
			count++
		}
		first := startSeq
		startSeq += count
		wg.Add(1)
		go func() {
			err := p.run(ctx, first, count, opts.window, message)
			if err != nil {
				select {
				case errors <- err:
				default:
				}
				cancel()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	cancel()
	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}

// run maintains a bounded pipeline; each reply frees one slot for another send.
func (p *peer) run(ctx context.Context, first, count, window int, message string) error {
	p.latencies = make([]time.Duration, 0, count)
	pending := make(map[int32]time.Time, min(window, count))
	next := first
	end := first + count
	request := profile.Greeting{Message: message}
	outgoing := cluster.Message{Route: profile.Greet, Data: &request}
	for next < end || len(pending) > 0 {
		for next < end && len(pending) < window {
			if err := ctx.Err(); err != nil {
				return err
			}
			seq := int32(next)
			pending[seq] = time.Now()
			p.attempted++
			outgoing.Seq = seq
			if err := p.push(&outgoing); err != nil {
				return fmt.Errorf("push seq %d: %w", seq, err)
			}
			p.sent++
			next++
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.overflow:
			return fmt.Errorf("reply queue overflow (unexpected or duplicate replies)")
		case r := <-p.replies:
			if r.err != nil {
				return r.err
			}
			start, ok := pending[r.seq]
			if !ok {
				return fmt.Errorf("unexpected or duplicate reply seq %d", r.seq)
			}
			delete(pending, r.seq)
			p.latencies = append(p.latencies, r.at.Sub(start))
		}
	}
	return nil
}

func printResult(peers []*peer, opts options, elapsed time.Duration) {
	latencies := make([]time.Duration, 0, opts.requests)
	var attempted, sent int
	for _, p := range peers {
		attempted += p.attempted
		sent += p.sent
		latencies = append(latencies, p.latencies...)
	}
	received := len(latencies)
	fmt.Printf("business: elapsed=%s target=%d attempted=%d sent=%d valid_replies=%d\n",
		elapsed, opts.requests, attempted, sent, received)
	fmt.Printf("unsent=%d push_errors=%d unconfirmed=%d TPS=%.0f message_MiB/s=%.2f\n",
		opts.requests-attempted, attempted-sent, sent-received,
		float64(received)/elapsed.Seconds(),
		float64(received)*float64(opts.size)*2/elapsed.Seconds()/(1024*1024))
	printLatency("RTT", latencies)
}

func printLatency(label string, values []time.Duration) {
	if len(values) == 0 {
		return
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	var total float64
	for _, value := range values {
		total += float64(value)
	}
	fmt.Printf("%s: min=%s avg=%s p50=%s p95=%s p99=%s max=%s\n",
		label, values[0], time.Duration(total/float64(len(values))),
		percentile(values, .50), percentile(values, .95), percentile(values, .99),
		values[len(values)-1])
}

func percentile(sorted []time.Duration, quantile float64) time.Duration {
	return sorted[max(0, int(math.Ceil(float64(len(sorted))*quantile))-1)]
}
