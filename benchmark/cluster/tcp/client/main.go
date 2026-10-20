package main

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/flag"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xrand"
)

const greet = 1

const (
	// modeThroughput pushes as fast as possible and reports capacity only. Requests are queued in
	// the client write path, so any latency measured here would be queueing time, not RTT.
	modeThroughput = "throughput"
	// modeLatency keeps a bounded per-connection window (closed loop) and reports RTT percentiles.
	modeLatency = "latency"
	// modeRate paces requests towards a target QPS and reports achieved rate and RTT percentiles.
	modeRate = "rate"
)

// Round phases. Senders push during the warmup and measure phases, park at the fence so the
// controller can drain in-flight requests and reset counters, and exit on stop.
const (
	phaseWarmup int32 = iota
	phaseFence
	phaseMeasure
	phaseStop
)

const (
	// sampleTotalCap bounds the total number of reservoir samples kept for latency statistics.
	sampleTotalCap = 200_000
	// samplePerConnMin keeps a usable sample size when many connections are used.
	samplePerConnMin = 100
	// fenceTimeout bounds the drain that precedes the measurement window.
	fenceTimeout = 10 * time.Second
	// drainTimeout bounds the drain that follows the measurement window.
	drainTimeout = 10 * time.Second
	// measureGuard is the hard cap of a latency round, which has no explicit duration.
	measureGuard = 10 * time.Minute
	// pollInterval is the polling period of the controller.
	pollInterval = 5 * time.Millisecond
	// interRoundPause separates consecutive rounds so the gate can release the sessions first.
	interRoundPause = time.Second
	// rateBatchPerSec is the number of pacing batches per second in rate mode.
	rateBatchPerSec = 200
)

type greetReq struct {
	Message string `json:"message"`
	SentAt  int64  `json:"sentAt"`
}

type greetRes struct {
	Message string `json:"message"`
	SentAt  int64  `json:"sentAt"`
}

type config struct {
	mode       string        // load model
	addr       string        // gate address
	conns      []int         // connection counts
	sizes      []int         // payload sizes
	duration   time.Duration // measurement window of the throughput and rate modes
	warmup     time.Duration // warmup window before every round
	requests   int           // total requests of a latency round
	window     int           // in-flight window per connection; 0 means the mode default
	rate       int           // total target QPS of a rate round
	repeat     int           // runs per configuration
	security   bool          // enable the session security
	out        string        // directory of the per-second series; empty disables the output
	cpuProfile string        // client CPU profile path; empty disables the profile
}

// connState is the per-connection benchmark state. The reply handler runs on the connection read
// goroutine, so the response buffer and the latency samples are only touched by a single goroutine
// and need no locking.
type connState struct {
	conn    *client.Conn
	req     greetReq
	res     greetRes
	window  chan struct{} // in-flight window; nil means unbounded
	samples []float64     // reservoir-sampled latency values (ms)
	seen    int64         // total latency observations
	limit   int           // reservoir capacity
}

// record samples a latency value with reservoir sampling, so memory stays bounded while the
// percentiles remain unbiased.
func (st *connState) record(latency float64) {
	st.seen++

	if st.limit <= 0 {
		return
	}

	if len(st.samples) < st.limit {
		st.samples = append(st.samples, latency)
		return
	}

	if idx := rand.Int64N(st.seen); idx < int64(st.limit) {
		st.samples[idx] = latency
	}
}

// release returns an in-flight window slot.
func (st *connState) release() {
	if st.window != nil {
		<-st.window
	}
}

type point struct {
	elapsed  float64
	tps      float64
	sent     int64
	recv     int64
	errors   int64
	inflight int64
}

// round holds the shared state of one benchmark round.
type round struct {
	message string
	window  int
	planned int64
	track   bool

	phase   atomic.Int32
	parked  atomic.Int64
	resume  chan struct{}
	pending atomic.Int64
	aborted atomic.Bool

	sent       atomic.Int64
	recv       atomic.Int64
	pushErrs   atomic.Int64
	parseErrs  atomic.Int64
	mismatches atomic.Int64
	warmupErrs int64

	startAt atomic.Int64
	endAt   atomic.Int64

	states map[*client.Conn]*connState
	setup  []float64
	series []point
}

type result struct {
	mode       string
	conns      int
	size       int
	window     int
	run        int
	repeats    int
	planned    int64
	duration   time.Duration
	sent       int64
	recv       int64
	pushErrs   int64
	parseErrs  int64
	mismatches int64
	warmupErrs int64
	drained    bool
	samples    int64
	latencies  []float64
	setup      []float64
	series     []point
	seriesPath string
	gcCycles   uint32
	allocBytes uint64
}

type runner struct {
	cfg     config
	proxy   *client.Proxy
	current atomic.Pointer[round]
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("invalid benchmark config: %v", err)
	}

	opts := make([]client.Option, 0, 2)
	opts = append(opts, client.WithClient(tcp.NewClient()))
	if cfg.security {
		opts = append(opts, client.WithSessionSecurity(ecc.NewSigner()))
	}

	component := client.NewClient(opts...)

	r := &runner{cfg: cfg, proxy: component.Proxy()}
	component.Proxy().AddHookListener(cluster.Start, r.start)
	component.Proxy().AddRouteHandler(greet, r.handleReply)

	container := due.NewContainer()
	container.Add(component)
	container.Serve(true)
}

func loadConfig() (config, error) {
	conns, err := parseInts(flag.String("conns", "50,100,200,500,1000"))
	if err != nil {
		return config{}, fmt.Errorf("invalid conns: %w", err)
	}

	sizes, err := parseInts(flag.String("size", "1024"))
	if err != nil {
		return config{}, fmt.Errorf("invalid size: %w", err)
	}

	cfg := config{
		// -mode is reserved by the framework (debug|test|pre-release|release), so the load model
		// is selected with -bench.
		mode:       flag.String("bench", modeThroughput),
		addr:       flag.String("addr", "127.0.0.1:3553"),
		conns:      conns,
		sizes:      sizes,
		duration:   flag.Duration("duration", 10*time.Second),
		warmup:     flag.Duration("warmup", 3*time.Second),
		requests:   flag.Int("requests", 100_000),
		window:     flag.Int("window", 0),
		rate:       flag.Int("rate", 100_000),
		repeat:     flag.Int("repeat", 1),
		security:   flag.Bool("security", false),
		out:        flag.String("out", ""),
		cpuProfile: flag.String("cpuprofile", ""),
	}

	switch cfg.mode {
	case modeThroughput, modeLatency, modeRate:
	default:
		return config{}, fmt.Errorf("unknown mode %q, want %s|%s|%s",
			cfg.mode, modeThroughput, modeLatency, modeRate)
	}

	if cfg.requests <= 0 {
		return config{}, fmt.Errorf("requests must be positive: %d", cfg.requests)
	}
	if cfg.window < 0 {
		return config{}, fmt.Errorf("window must not be negative: %d", cfg.window)
	}
	if cfg.repeat <= 0 {
		return config{}, fmt.Errorf("repeat must be positive: %d", cfg.repeat)
	}
	if cfg.mode == modeRate && cfg.rate <= 0 {
		return config{}, fmt.Errorf("rate must be positive in rate mode: %d", cfg.rate)
	}

	return cfg, nil
}

// window returns the effective per-connection in-flight window of the current mode.
func (r *runner) window() int {
	if r.cfg.window > 0 {
		return r.cfg.window
	}

	switch r.cfg.mode {
	case modeLatency:
		return 1
	case modeRate:
		return 32
	default:
		return 0
	}
}

// start is the client start hook. The container waits for this hook to return, so the whole
// benchmark runs inside it and the process shuts the client down when it completes.
func (r *runner) start(_ *client.Proxy) {
	stopProfile := r.startProfile()
	defer stopProfile()

	if r.cfg.out != "" {
		if err := os.MkdirAll(r.cfg.out, 0o755); err != nil {
			log.Fatalf("create output directory failed: %v", err)
		}
	}

	r.printEnvironment()

	results := make([]*result, 0, len(r.cfg.sizes)*len(r.cfg.conns)*r.cfg.repeat)

	for _, size := range r.cfg.sizes {
		for _, conns := range r.cfg.conns {
			for run := 1; run <= r.cfg.repeat; run++ {
				res, err := r.runRound(conns, size, run)
				if err != nil {
					log.Errorf("round failed, conns: %d size: %d run: %d err: %v", conns, size, run, err)
					continue
				}

				printReport(res, r.cfg)
				results = append(results, res)
			}
		}
	}

	printSummary(results)
}

func (r *runner) startProfile() func() {
	if r.cfg.cpuProfile == "" {
		return func() {}
	}

	file, err := os.Create(r.cfg.cpuProfile)
	if err != nil {
		log.Errorf("create cpu profile failed: %v", err)
		return func() {}
	}

	if err = pprof.StartCPUProfile(file); err != nil {
		log.Errorf("start cpu profile failed: %v", err)
		_ = file.Close()
		return func() {}
	}

	return func() {
		pprof.StopCPUProfile()
		_ = file.Close()
	}
}

// runRound dials the connections, warms them up, measures one steady-state window and collects the
// results. Every wait is bounded, so a lost reply is reported instead of hanging the process.
func (r *runner) runRound(conns, size, run int) (*result, error) {
	mode := r.cfg.mode
	window := r.window()

	rd := &round{
		message: xrand.Letters(size),
		window:  window,
		track:   mode != modeThroughput,
		states:  make(map[*client.Conn]*connState, conns),
		resume:  make(chan struct{}),
		setup:   make([]float64, 0, conns),
	}
	rd.phase.Store(phaseWarmup)

	limit := max(samplePerConnMin, sampleTotalCap/conns)

	for range conns {
		dialStart := time.Now()
		conn, err := r.proxy.Dial(client.WithDialAddr(r.cfg.addr))
		if err != nil {
			rd.closeConns()
			return nil, fmt.Errorf("dial %s failed: %w", r.cfg.addr, err)
		}

		st := &connState{conn: conn, limit: limit}
		st.req.Message = rd.message
		if window > 0 {
			st.window = make(chan struct{}, window)
		}

		rd.setup = append(rd.setup, float64(time.Since(dialStart))/float64(time.Millisecond))
		rd.states[conn] = st
	}

	// Publishing the round publishes the connection states as well: the reply handler only reads
	// the map after loading the pointer, and the map is never mutated afterwards.
	r.current.Store(rd)

	quota, rate := 0, 0
	switch mode {
	case modeLatency:
		quota = (r.cfg.requests + conns - 1) / conns
		rd.planned = int64(quota) * int64(conns)
	case modeRate:
		rate = max(1, r.cfg.rate/conns)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var senders sync.WaitGroup
	for _, st := range rd.states {
		senders.Add(1)
		go func(st *connState) {
			defer senders.Done()
			r.sendLoop(ctx, rd, st, quota, rate)
		}(st)
	}

	if r.cfg.warmup > 0 {
		time.Sleep(r.cfg.warmup)
	}

	// Fence the senders, then wait until every request of the warmup has been completed. Only then
	// may the counters and the latency samples be reset.
	rd.phase.Store(phaseFence)

	if !waitCond(fenceTimeout, func() bool {
		return rd.parked.Load() == int64(conns) || rd.aborted.Load()
	}) {
		return nil, r.abortRound(rd, cancel, &senders, "senders did not park at the fence")
	}
	if rd.aborted.Load() {
		return nil, r.abortRound(rd, cancel, &senders, "push failed during warmup")
	}
	if !waitCond(fenceTimeout, func() bool { return rd.pending.Load() == 0 }) {
		return nil, r.abortRound(rd, cancel, &senders, "warmup requests did not drain")
	}

	rd.warmupErrs = rd.pushErrs.Load() + rd.parseErrs.Load() + rd.mismatches.Load()
	if rd.warmupErrs > 0 {
		return nil, r.abortRound(rd, cancel, &senders, "warmup produced invalid responses")
	}

	rd.reset()

	runtime.GC()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	rd.startAt.Store(time.Now().UnixNano())
	stopSampler := r.startSampler(rd)

	rd.phase.Store(phaseMeasure)
	close(rd.resume)

	switch mode {
	// The timestamp of a latency round is pushed by the client, so the round is bounded by the
	// hard cap instead of a fixed duration.
	case modeLatency:
		waitCond(measureGuard, func() bool {
			return rd.sent.Load() >= rd.planned || rd.aborted.Load()
		})
	default:
		deadline := time.Now().Add(r.cfg.duration)
		for time.Now().Before(deadline) && !rd.aborted.Load() {
			time.Sleep(pollInterval)
		}
	}

	rd.phase.Store(phaseStop)
	stopAt := time.Now()
	cancel()
	waitSenders(&senders, drainTimeout)

	drained := waitCond(drainTimeout, func() bool { return rd.pending.Load() == 0 })

	if drained {
		rd.endAt.Store(time.Now().UnixNano())
	} else {
		// A timed out drain means the remaining requests are lost; counting the drain wait as
		// measurement time would dilute the throughput of an already invalid round.
		rd.endAt.Store(stopAt.UnixNano())
	}
	stopSampler()

	runtime.ReadMemStats(&after)

	r.current.Store(nil)
	rd.closeConns()
	time.Sleep(interRoundPause)

	res := &result{
		mode:       mode,
		conns:      conns,
		size:       size,
		window:     window,
		run:        run,
		repeats:    r.cfg.repeat,
		planned:    rd.planned,
		duration:   time.Duration(rd.endAt.Load() - rd.startAt.Load()),
		sent:       rd.sent.Load(),
		recv:       rd.recv.Load(),
		pushErrs:   rd.pushErrs.Load(),
		parseErrs:  rd.parseErrs.Load(),
		mismatches: rd.mismatches.Load(),
		warmupErrs: rd.warmupErrs,
		drained:    drained,
		setup:      rd.setup,
		series:     rd.series,
		gcCycles:   after.NumGC - before.NumGC,
		allocBytes: after.TotalAlloc - before.TotalAlloc,
	}
	res.latencies, res.samples = rd.mergeLatencies()
	sort.Float64s(res.latencies)

	if r.cfg.out != "" {
		path, err := writeSeries(r.cfg.out, res)
		if err != nil {
			log.Errorf("write series failed: %v", err)
		} else {
			res.seriesPath = path
		}
	}

	return res, nil
}

// abortRound stops the senders of an invalid round and releases its connections.
func (r *runner) abortRound(rd *round, cancel context.CancelFunc, senders *sync.WaitGroup, reason string) error {
	rd.phase.Store(phaseStop)
	cancel()
	waitSenders(senders, drainTimeout)

	r.current.Store(nil)
	rd.closeConns()
	time.Sleep(interRoundPause)

	return fmt.Errorf("%s, pending: %d", reason, rd.pending.Load())
}

// sendLoop pushes requests of one connection until the round stops or its quota is reached.
func (r *runner) sendLoop(ctx context.Context, rd *round, st *connState, quota, rate int) {
	var (
		seq      int32
		measured int
		batch    = 1
		step     time.Duration
		next     time.Time
		timer    *time.Timer
	)

	if rate > 0 {
		batch = max(1, rate/rateBatchPerSec)
		step = time.Duration(float64(time.Second) * float64(batch) / float64(rate))
		timer = time.NewTimer(time.Hour)
		if !timer.Stop() {
			<-timer.C
		}
		defer timer.Stop()
		next = time.Now()
	}

	for {
		for i := 0; i < batch; i++ {
			if quota > 0 && measured >= quota {
				return
			}

			if !waitReady(ctx, rd) {
				return
			}

			if st.window != nil {
				select {
				case st.window <- struct{}{}:
				case <-ctx.Done():
					return
				}

				// The phase may have changed while waiting for a window slot. Release the slot
				// and let the next iteration park at the fence or exit.
				if phase := rd.phase.Load(); phase != phaseWarmup && phase != phaseMeasure {
					st.release()
					continue
				}
				if quota > 0 && measured >= quota {
					st.release()
					return
				}
			}

			seq++
			st.req.SentAt = time.Now().UnixNano()
			rd.pending.Add(1)

			if err := st.conn.Push(&cluster.Message{Route: greet, Seq: seq, Data: &st.req}); err != nil {
				rd.pending.Add(-1)
				rd.pushErrs.Add(1)
				rd.aborted.Store(true)
				st.release()
				return
			}

			rd.sent.Add(1)

			if rd.phase.Load() == phaseMeasure {
				measured++
			}
		}

		if timer != nil {
			next = next.Add(step)
			if delay := time.Until(next); delay > 0 {
				timer.Reset(delay)
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
			} else {
				// The schedule was left behind by a stall or by the warmup fence: restart it
				// instead of bursting the whole backlog at once.
				next = time.Now()
			}
		}
	}
}

// waitReady reports whether the round accepts new requests. It parks the caller at the fence and
// returns false once the round stops or the context is cancelled.
func waitReady(ctx context.Context, rd *round) bool {
	for {
		switch rd.phase.Load() {
		case phaseWarmup, phaseMeasure:
			return true
		case phaseFence:
			rd.parked.Add(1)
			select {
			case <-rd.resume:
				rd.parked.Add(-1)
				return true
			case <-ctx.Done():
				rd.parked.Add(-1)
				return false
			}
		default:
			return false
		}
	}
}

// handleReply completes one request: it validates the echo, records the latency and releases the
// in-flight accounting. It runs on the connection read goroutine.
func (r *runner) handleReply(ctx *client.Context) {
	rd := r.current.Load()
	if rd == nil {
		return
	}

	st, ok := rd.states[ctx.Conn()]
	if !ok {
		return
	}

	if err := ctx.Parse(&st.res); err != nil {
		rd.parseErrs.Add(1)
		rd.finish(st)
		return
	}

	// The node echoes both the payload and the send timestamp. A zero timestamp means the node is
	// not echoing it, which would silently distort the latency statistics.
	if st.res.Message != rd.message || st.res.SentAt == 0 {
		rd.mismatches.Add(1)
		rd.finish(st)
		return
	}

	if rd.track {
		st.record(float64(time.Now().UnixNano()-st.res.SentAt) / 1e6)
	}

	rd.recv.Add(1)
	rd.finish(st)
}

func (rd *round) finish(st *connState) {
	st.release()
	rd.pending.Add(-1)
}

func (rd *round) reset() {
	rd.sent.Store(0)
	rd.recv.Store(0)
	rd.pushErrs.Store(0)
	rd.parseErrs.Store(0)
	rd.mismatches.Store(0)
	rd.series = rd.series[:0]

	for _, st := range rd.states {
		st.samples = st.samples[:0]
		st.seen = 0
	}
}

func (rd *round) mergeLatencies() ([]float64, int64) {
	if !rd.track {
		return nil, 0
	}

	var (
		total int
		seen  int64
	)

	for _, st := range rd.states {
		total += len(st.samples)
		seen += st.seen
	}

	merged := make([]float64, 0, total)
	for _, st := range rd.states {
		merged = append(merged, st.samples...)
	}

	return merged, seen
}

func (rd *round) closeConns() {
	for conn := range rd.states {
		_ = conn.Close()
	}
}

// startSampler records a per-second series of the measurement window. Collection is only enabled
// when the run writes a CSV, so the hot path is not charged for unused samples.
func (r *runner) startSampler(rd *round) func() {
	if r.cfg.out == "" {
		return func() {}
	}

	var (
		stop = make(chan struct{})
		done = make(chan struct{})
	)

	go func() {
		defer close(done)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		start := time.Unix(0, rd.startAt.Load())
		var prevSent, prevRecv, prevErrs int64

		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				sent := rd.sent.Load()
				recv := rd.recv.Load()
				errs := rd.pushErrs.Load() + rd.parseErrs.Load() + rd.mismatches.Load()

				rd.series = append(rd.series, point{
					elapsed:  now.Sub(start).Seconds(),
					tps:      float64(recv - prevRecv),
					sent:     sent - prevSent,
					recv:     recv - prevRecv,
					errors:   errs - prevErrs,
					inflight: rd.pending.Load(),
				})

				prevSent, prevRecv, prevErrs = sent, recv, errs
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func writeSeries(dir string, res *result) (string, error) {
	name := fmt.Sprintf("%s-c%d-s%d-run%d.csv", res.mode, res.conns, res.size, res.run)
	path := filepath.Join(dir, name)

	file, err := os.Create(path)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("elapsed_s,tps,sent,recv,errors,inflight\n")

	for _, p := range res.series {
		fmt.Fprintf(&b, "%.3f,%.0f,%d,%d,%d,%d\n", p.elapsed, p.tps, p.sent, p.recv, p.errors, p.inflight)
	}

	if _, err = file.WriteString(b.String()); err != nil {
		_ = file.Close()
		return "", err
	}

	return path, file.Close()
}

func waitCond(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)

	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

func waitSenders(senders *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})

	go func() {
		senders.Wait()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func (r *runner) printEnvironment() {
	fmt.Printf("\n%s\n", sectionLineWith("ENVIRONMENT", '='))
	fmt.Printf("  Go:                  %s\n", runtime.Version())
	fmt.Printf("  CPU / GOMAXPROCS:    %d / %d\n", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	fmt.Printf("  GOGC:                %s\n", goGC())
	fmt.Printf("  Codec:               %s\n", etc.Get("etc.cluster.client.codec", "proto").String())
	fmt.Printf("  Load:                %s\n", r.cfg.mode)
	fmt.Printf("  Target:              %s\n", r.cfg.addr)
	fmt.Printf("  Security:            %t\n", r.cfg.security)
	fmt.Printf("  Warmup:              %s\n", r.cfg.warmup)
	fmt.Printf("%s\n", sectionLineWith("END", '='))
}

func goGC() string {
	if value := os.Getenv("GOGC"); value != "" {
		return value
	}

	return "100"
}

func printReport(res *result, cfg config) {
	window := "unbounded"
	if res.window > 0 {
		window = strconv.Itoa(res.window)
	}

	tps := res.tps()
	bandwidth := 0.0
	if seconds := res.duration.Seconds(); seconds > 0 {
		bandwidth = float64(res.recv*int64(res.size)*2) / seconds / (1024 * 1024)
	}
	lost := res.lost()

	fmt.Printf("\n%s\n", sectionLineWith("TCP BENCHMARK", '='))
	fmt.Printf("  Mode:                %s\n", res.mode)
	fmt.Printf("  Concurrency:         %d conns\n", res.conns)
	fmt.Printf("  Window:              %s\n", window)
	fmt.Printf("  Payload:             %s (business data, echoed with a timestamp field)\n", convBytes(res.size))
	fmt.Printf("  Target:              %s\n", cfg.addr)
	fmt.Printf("  Security:            %t\n", cfg.security)
	fmt.Printf("  Duration:            %.3fs\n", res.duration.Seconds())
	if res.planned > 0 {
		fmt.Printf("  Requests:            %s sent / %s planned\n", formatNum(res.sent), formatNum(res.planned))
	} else {
		fmt.Printf("  Requests:            %s sent\n", formatNum(res.sent))
	}
	if !res.drained {
		fmt.Printf("  Warning:             incomplete drain, the round is invalid and the duration\n")
		fmt.Printf("                       stops at the measurement deadline\n")
	}
	fmt.Printf("%s\n", sectionLine("Throughput"))
	fmt.Printf("  TPS:                 %s resp/s\n", formatNum(tps))
	fmt.Printf("  Bandwidth:           %.2f MB/s (payload, send+recv)\n", bandwidth)

	if res.mode == modeThroughput {
		fmt.Printf("%s\n", sectionLine("Latency"))
		fmt.Printf("  n/a                  open-loop mode queues requests, so the delay would be\n")
		fmt.Printf("                       queueing time instead of RTT\n")
	} else {
		printLatency(res)
	}

	fmt.Printf("%s\n", sectionLine("Reliability"))
	fmt.Printf("  Success Rate:        %.2f%%\n", res.successRate())
	fmt.Printf("  Sent / Recv:         %s / %s\n", formatNum(res.sent), formatNum(res.recv))
	fmt.Printf("  Lost:                %s\n", formatNum(lost))
	fmt.Printf("  Push / Parse / Bad:  %d / %d / %d\n", res.pushErrs, res.parseErrs, res.mismatches)
	fmt.Printf("  Warmup Errors:       %d\n", res.warmupErrs)
	fmt.Printf("  Drained:             %t\n", res.drained)

	fmt.Printf("%s\n", sectionLine("Connection"))
	fmt.Printf("  Connections:         %d\n", res.conns)
	if len(res.setup) > 0 {
		min, avg, max := connStats(res.setup)
		fmt.Printf("  Setup Min/Avg/Max:   %.3f / %.3f / %.3f ms\n", min, avg, max)
	}
	fmt.Printf("  GC Cycles / Alloc:   %d / %.1f MB\n", res.gcCycles, float64(res.allocBytes)/(1024*1024))
	if res.seriesPath != "" {
		fmt.Printf("  Series:              %s (%d rows)\n", res.seriesPath, len(res.series))
	}
	fmt.Printf("%s\n", sectionLineWith("END", '='))
}

func printLatency(res *result) {
	fmt.Printf("%s\n", sectionLine("Latency (ms, client-side RTT)"))

	if len(res.latencies) == 0 {
		fmt.Printf("  no samples\n")
		return
	}

	min, max, avg, std := latencyStats(res.latencies)

	fmt.Printf("  Samples:             %s kept / %s observed\n", formatNum(int64(len(res.latencies))), formatNum(res.samples))
	fmt.Printf("  Min:                 %.3f\n", min)
	fmt.Printf("  Max:                 %.3f\n", max)
	fmt.Printf("  Avg:                 %.3f\n", avg)
	fmt.Printf("  StdDev:              %.3f\n", std)
	fmt.Printf("  P50:                 %.3f\n", percentile(res.latencies, 0.50))
	fmt.Printf("  P75:                 %.3f\n", percentile(res.latencies, 0.75))
	fmt.Printf("  P90:                 %.3f\n", percentile(res.latencies, 0.90))
	fmt.Printf("  P95:                 %.3f\n", percentile(res.latencies, 0.95))
	fmt.Printf("  P99:                 %.3f\n", percentile(res.latencies, 0.99))
	fmt.Printf("  P999:                %.3f\n", percentile(res.latencies, 0.999))
}

func printSummary(results []*result) {
	if len(results) == 0 {
		return
	}
	if len(results) == 1 && results[0].repeats == 1 {
		return
	}

	type key struct {
		mode  string
		conns int
		size  int
	}

	var (
		groups = make(map[key][]*result)
		keys   = make([]key, 0, len(results))
	)

	for _, res := range results {
		k := key{mode: res.mode, conns: res.conns, size: res.size}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], res)
	}

	fmt.Printf("\n%s\n", sectionLineWith("SUMMARY", '='))
	fmt.Printf("  %-10s %-6s %-8s %-4s %-12s %-9s %-9s %-8s\n",
		"MODE", "CONNS", "SIZE", "RUN", "TPS", "P50(ms)", "P99(ms)", "SUCCESS")

	for _, k := range keys {
		group := groups[k]

		for _, res := range group {
			fmt.Printf("  %-10s %-6d %-8s %-4d %-12s %-9.3f %-9.3f %-8.2f%%\n",
				res.mode, res.conns, convBytes(res.size), res.run,
				formatNum(res.tps()), res.p50(), res.p99(), res.successRate())
		}

		if len(group) < 2 {
			continue
		}

		tpsValues := make([]float64, 0, len(group))
		p50Values := make([]float64, 0, len(group))
		p99Values := make([]float64, 0, len(group))
		for _, res := range group {
			tpsValues = append(tpsValues, float64(res.tps()))
			p50Values = append(p50Values, res.p50())
			p99Values = append(p99Values, res.p99())
		}

		fmt.Printf("  %-10s %-6d %-8s %-4s %-12s %-9.3f %-9.3f %-8s\n",
			k.mode, k.conns, convBytes(k.size), "med",
			formatNum(int64(median(tpsValues))), median(p50Values), median(p99Values), "-")
	}

	fmt.Printf("%s\n\n", sectionLineWith("END", '='))
}

func (res *result) tps() int64 {
	seconds := res.duration.Seconds()
	if seconds <= 0 {
		return 0
	}

	return int64(float64(res.recv) / seconds)
}

func (res *result) successRate() float64 {
	if res.sent == 0 {
		return 0
	}

	return float64(res.recv) / float64(res.sent) * 100
}

func (res *result) lost() int64 {
	return max(0, res.sent-res.recv-res.parseErrs-res.mismatches)
}

func (res *result) p50() float64 {
	return percentile(res.latencies, 0.50)
}

func (res *result) p99() float64 {
	return percentile(res.latencies, 0.99)
}

// sectionLine returns a separator line with title centered in the middle.
func sectionLine(title string) string {
	return sectionLineWith(title, '-')
}

// sectionLineWith returns a separator line with title centered in the middle, using ch as the fill
// character.
func sectionLineWith(title string, ch byte) string {
	const width = 64
	label := " " + title + " "
	n := width - len(label)
	left := n / 2
	right := n - left
	return strings.Repeat(string(ch), left) + label + strings.Repeat(string(ch), right)
}

// latencyStats computes the min, max, average and standard deviation of the latency values.
func latencyStats(data []float64) (min, max, avg, std float64) {
	if len(data) == 0 {
		return 0, 0, 0, 0
	}

	min, max = data[0], data[0]
	sum := 0.0
	for _, v := range data {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg = sum / float64(len(data))

	sumSq := 0.0
	for _, v := range data {
		sumSq += (v - avg) * (v - avg)
	}
	std = math.Sqrt(sumSq / float64(len(data)))

	return
}

// percentile computes the p-th percentile using linear interpolation.
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	if p <= 0 {
		return data[0]
	}
	if p >= 1 {
		return data[len(data)-1]
	}

	pos := p * float64(len(data)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return data[lower]
	}

	frac := pos - float64(lower)
	return data[lower]*(1-frac) + data[upper]*frac
}

// median returns the median of the given values without mutating the input.
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return sorted[len(sorted)/2]
}

// connStats computes the min, average and max connection setup time.
func connStats(durations []float64) (min, avg, max float64) {
	if len(durations) == 0 {
		return 0, 0, 0
	}

	min, max = durations[0], durations[0]
	sum := 0.0
	for _, d := range durations {
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	return min, sum / float64(len(durations)), max
}

// formatNum formats a large number with thousands separators.
func formatNum(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	s := fmt.Sprintf("%d", n)
	var result []byte
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		if len(result) > 0 {
			result = append([]byte(s[start:i]+","), result...)
		} else {
			result = []byte(s[start:i])
		}
	}
	return string(result)
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

func parseInts(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	values := make([]int, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", part, err)
		}
		if v <= 0 {
			return nil, fmt.Errorf("value must be positive: %d", v)
		}

		values = append(values, v)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no value given")
	}

	return values, nil
}
