package gate

import (
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// guardConn is a test connection with a configurable peer IP.
type guardConn struct {
	*gateConn
	ip string
}

// newGuardConn returns a test connection with the given id and peer IP.
func newGuardConn(id int64, ip string) *guardConn {
	return &guardConn{gateConn: newGateConn(id, 0), ip: ip}
}

// RemoteIP returns the configured peer IP.
func (c *guardConn) RemoteIP() (string, error) {
	return c.ip, nil
}

// recycledConn simulates a transport connection object that is reused for a newer connection.
type recycledConn struct {
	*gateConn
	currentID atomic.Int64
	closeIDs  chan int64
}

// newRecycledConn returns a test connection whose object may be reused with another ID.
func newRecycledConn(id int64) *recycledConn {
	c := &recycledConn{gateConn: newGateConn(id, 0), closeIDs: make(chan int64, 1)}
	c.currentID.Store(id)
	return c
}

// ID returns the current ID of the connection object.
func (c *recycledConn) ID() int64 {
	return c.currentID.Load()
}

// CloseIf records the close attempt and only closes the connection when the ID still matches.
func (c *recycledConn) CloseIf(id int64) error {
	select {
	case c.closeIDs <- id:
	default:
	}

	if c.ID() != id {
		return errors.ErrConnectionClosed
	}

	return c.gateConn.Close(true)
}

func TestIPKey(t *testing.T) {
	tests := []struct {
		name string
		give string
		want string
		ok   bool
	}{
		{name: "ipv4", give: "192.168.1.10", want: "192.168.1.10/32", ok: true},
		{name: "ipv4 mapped ipv6", give: "::ffff:192.168.1.10", want: "192.168.1.10/32", ok: true},
		{name: "ipv6 aggregated by 64", give: "2001:db8:1:2::1", want: "2001:db8:1:2::/64", ok: true},
		{name: "ipv6 prefix", give: "2001:db8:1:2:3:4:5:6", want: "2001:db8:1:2::/64", ok: true},
		{name: "ipv6 zone dropped", give: "fe80::1%eth0", want: "fe80::/64", ok: true},
		{name: "invalid", give: "not-an-ip", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, ok := parseIP(tt.give)
			if ok != tt.ok {
				t.Fatalf("unexpected parse result: %v", ok)
			}
			if ok {
				if got := ipKey(addr).String(); got != tt.want {
					t.Fatalf("unexpected key: %s, want: %s", got, tt.want)
				}
			}
		})
	}
}

func TestIPKeyAllocations(t *testing.T) {
	addr, ok := parseIP("192.168.1.10")
	if !ok {
		t.Fatal("invalid test IP")
	}

	g := newGuard(&guardOptions{})
	key := ipKey(addr)

	if allocs := testing.AllocsPerRun(1000, func() { _, _ = parseIP("192.168.1.10") }); allocs != 0 {
		t.Fatalf("parseIP must not allocate: %v", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() { _ = ipKey(addr) }); allocs != 0 {
		t.Fatalf("ipKey must not allocate: %v", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() { _ = g.loadShard(key) }); allocs != 0 {
		t.Fatalf("loadShard must not allocate: %v", allocs)
	}
}

func TestGateGuardOptions(t *testing.T) {
	o := defaultOptions()

	if o.guard.strikeThreshold != defaultGuardStrikeThreshold {
		t.Fatalf("unexpected default strike threshold: %d", o.guard.strikeThreshold)
	}
	if o.guard.maxEntries != defaultGuardMaxEntries {
		t.Fatalf("unexpected default max entries: %d", o.guard.maxEntries)
	}

	WithGuardMaxConns(3)(o)
	WithGuardMaxEntries(1024)(o)
	WithGuardAcceptRate(2, time.Minute)(o)
	WithGuardStrikeThreshold(4)(o)
	WithGuardStrikeWindow(10 * time.Minute)(o)
	WithGuardMinBanDuration(2 * time.Minute)(o)
	WithGuardMaxBanDuration(30 * time.Minute)(o)
	WithGuardWhitelist([]string{"10.0.0.0/8"})(o)

	g := newGuard(o.guard)

	if !g.enabled {
		t.Fatal("the guard must be enabled")
	}
	if g.opts.maxConns != 3 {
		t.Fatalf("unexpected max connections: %d", g.opts.maxConns)
	}
	if g.opts.maxEntries != 1024 {
		t.Fatalf("unexpected max entries: %d", g.opts.maxEntries)
	}
	if g.opts.quota != 2 || g.opts.period != time.Minute {
		t.Fatalf("unexpected admission rate: quota=%v, period=%v",
			g.opts.quota, g.opts.period)
	}
	if g.opts.burst != 2 {
		t.Fatalf("unexpected admission burst: %d", g.opts.burst)
	}
	if g.opts.strikeThreshold != 4 || g.opts.strikeWindow != 10*time.Minute {
		t.Fatalf("unexpected ban policy: threshold=%d, window=%v",
			g.opts.strikeThreshold, g.opts.strikeWindow)
	}
	if g.opts.minBanDuration != 2*time.Minute || g.opts.maxBanDuration != 30*time.Minute {
		t.Fatalf("unexpected ban durations: base=%v, max=%v",
			g.opts.minBanDuration, g.opts.maxBanDuration)
	}
	if len(g.whitelist) != 1 || g.whitelist[0].String() != "10.0.0.0/8" {
		t.Fatalf("unexpected whitelist: %v", g.whitelist)
	}
	if len(o.guard.whitelist) != 1 {
		t.Fatal("the guard must not mutate the caller's options")
	}

	// Later option changes must not affect the guard's owned configuration.
	WithGuardStrikeThreshold(0)(o)
	if o.guard.strikeThreshold != 0 {
		t.Fatalf("a zero strike threshold must disable bans: %d", o.guard.strikeThreshold)
	}
	if g.opts.strikeThreshold != 4 {
		t.Fatalf("the guard must own its configuration: %d", g.opts.strikeThreshold)
	}
}

func TestGuardIPMaxConns(t *testing.T) {
	g := newGuard(&guardOptions{maxConns: 1, strikeThreshold: 0})

	first := newGuardConn(1, "10.0.0.1")
	second := newGuardConn(2, "10.0.0.1")
	other := newGuardConn(3, "10.0.0.2")

	if !g.admit(first) {
		t.Fatal("the first connection must be admitted")
	}
	if g.admit(second) {
		t.Fatal("the second connection from the same IP must be rejected")
	}
	waitForCondition(t, time.Second, second.closed.Load, "the rejected connection must be closed")

	if got := g.rejectedLimit.Load(); got != 1 {
		t.Fatalf("unexpected rejected connection count: %d", got)
	}

	if !g.admit(other) {
		t.Fatal("a connection from another IP must be admitted")
	}

	g.remove(first)

	reused := newGuardConn(4, "10.0.0.1")
	if !g.admit(reused) {
		t.Fatal("the IP slot must be released after the connection is removed")
	}
}

func TestGuardIPAcceptRate(t *testing.T) {
	g := newGuard(&guardOptions{
		quota:           1,
		period:          time.Hour,
		burst:           1,
		strikeThreshold: 0,
	})

	first := newGuardConn(1, "10.0.0.1")
	if !g.admit(first) {
		t.Fatal("the first connection must be admitted")
	}
	g.remove(first)

	second := newGuardConn(2, "10.0.0.1")
	if g.admit(second) {
		t.Fatal("the connection beyond the admission rate must be rejected")
	}
	waitForCondition(t, time.Second, second.closed.Load, "the rejected connection must be closed")

	if got := g.rejectedRate.Load(); got != 1 {
		t.Fatalf("unexpected rejected admission count: %d", got)
	}
}

func TestGuardDisabled(t *testing.T) {
	g := newGuard(&guardOptions{})

	conn := newGuardConn(1, "10.0.0.1")
	if !g.admit(conn) {
		t.Fatal("a disabled guard must admit the connection")
	}
	if got := g.entries.Load(); got != 0 {
		t.Fatalf("a disabled guard must not track entries: %d", got)
	}
	if g.strike(conn) {
		t.Fatal("a disabled guard must not ban")
	}

	g.remove(conn)
	if got := g.entries.Load(); got != 0 {
		t.Fatalf("a disabled guard must not track entries: %d", got)
	}
}

func TestGuardSkipsConnTrackingWithoutLimits(t *testing.T) {
	g := newGuard(&guardOptions{quota: 1, period: time.Hour, burst: 1})

	conn := newGuardConn(1, "10.0.0.1")
	if !g.admit(conn) {
		t.Fatal("the connection must be admitted")
	}

	addr, ok := parseIP("10.0.0.1")
	if !ok {
		t.Fatal("invalid test IP")
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	shard.mu.Lock()
	entry := shard.entries[key]
	shard.mu.Unlock()

	if entry == nil {
		t.Fatal("the entry must be created")
	}
	if entry.conns != nil {
		t.Fatal("the guard must not track connections without limits or bans")
	}
}

func TestGuardMemoryLayout(t *testing.T) {
	const cacheLine = 64

	if size := unsafe.Sizeof(guardShard{}); size%cacheLine != 0 {
		t.Fatalf("guardShard size %d is not a multiple of the cache line size", size)
	}

	var g guard

	offsets := []uintptr{
		unsafe.Offsetof(g.entries),
		unsafe.Offsetof(g.rejectedBan),
		unsafe.Offsetof(g.rejectedLimit),
		unsafe.Offsetof(g.rejectedRate),
		unsafe.Offsetof(g.untracked),
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i]-offsets[i-1] < cacheLine {
			t.Fatalf("guard counters share a cache line: %d and %d", offsets[i-1], offsets[i])
		}
	}
}

func TestGuardRateLimitedDependency(t *testing.T) {
	g := newGuard(&guardOptions{strikeThreshold: 2})
	if !g.enabled {
		t.Fatal("a direct guard assumes the limiter is enabled")
	}

	g.setRateLimited(false)
	if g.enabled {
		t.Fatal("a strike-only guard must be disabled without a limiter")
	}

	g.opts.maxConns = 1
	g.setRateLimited(false)
	if !g.enabled {
		t.Fatal("an IP limit must enable the guard without a limiter")
	}
}

func TestGuardEntryLimit(t *testing.T) {
	g := newGuard(&guardOptions{maxConns: 1})
	g.maxEntries = 1

	first := newGuardConn(1, "10.0.0.1")
	second := newGuardConn(2, "10.0.0.2")

	if !g.admit(first) {
		t.Fatal("the first connection must be admitted")
	}
	if !g.admit(second) {
		t.Fatal("a full entry table must fail open")
	}
	if got := g.entries.Load(); got != 1 {
		t.Fatalf("unexpected entry count: %d", got)
	}
	if got := g.untracked.Load(); got != 1 {
		t.Fatalf("unexpected untracked count: %d", got)
	}

	addr, ok := parseIP("10.0.0.2")
	if !ok {
		t.Fatal("invalid test IP")
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	shard.mu.Lock()
	_, tracked := shard.entries[key]
	shard.mu.Unlock()

	if tracked {
		t.Fatal("an IP beyond the entry limit must not be tracked")
	}
}

func TestGuardSweepReleasesEntry(t *testing.T) {
	g := newGuard(&guardOptions{maxConns: 1})

	conn := newGuardConn(1, "10.0.0.1")
	if !g.admit(conn) {
		t.Fatal("the connection must be admitted")
	}
	g.remove(conn)

	addr, ok := parseIP("10.0.0.1")
	if !ok {
		t.Fatal("invalid test IP")
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	shard.mu.Lock()
	shard.entries[key].lastSeen = g.now() - int64(defaultGuardEntryTTL) - 1
	shard.mu.Unlock()

	g.sweep()

	if got := g.entries.Load(); got != 0 {
		t.Fatalf("the swept entry must be released: %d", got)
	}
}

func TestGuardSweepRetainsBanHistory(t *testing.T) {
	g := newGuard(&guardOptions{
		strikeThreshold: 1,
		strikeWindow:    time.Minute,
		minBanDuration:  time.Minute,
		maxBanDuration:  time.Hour,
	})

	conn := newGuardConn(1, "10.0.0.1")

	addr, ok := parseIP("10.0.0.1")
	if !ok {
		t.Fatal("invalid test IP")
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	if !g.admit(conn) {
		t.Fatal("the connection must be admitted")
	}
	if !g.strike(conn) {
		t.Fatal("the strike must trigger a ban")
	}

	guardExpireBan(g, "10.0.0.1")

	shard.mu.Lock()
	shard.entries[key].lastSeen = g.now() - int64(defaultGuardEntryTTL) - 1
	shard.mu.Unlock()

	g.sweep()

	shard.mu.Lock()
	_, retained := shard.entries[key]
	shard.mu.Unlock()

	if !retained {
		t.Fatal("an entry with recent ban history must be retained")
	}

	// Expire the escalation window as well; the entry can now be released.
	shard.mu.Lock()
	shard.entries[key].lastBanAt = g.now() - int64(time.Hour) - 1
	shard.mu.Unlock()

	g.sweep()

	if got := g.entries.Load(); got != 0 {
		t.Fatalf("the expired entry must be released: %d", got)
	}
}

func TestGuardStrikeBanEscalation(t *testing.T) {
	g := newGuard(&guardOptions{
		strikeThreshold: 1,
		strikeWindow:    time.Minute,
		minBanDuration:  time.Minute,
		maxBanDuration:  time.Hour,
	})

	conn := newGuardConn(1, "10.0.0.1")
	if !g.admit(conn) {
		t.Fatal("the connection must be admitted")
	}

	if !g.strike(conn) {
		t.Fatal("the strike must trigger a ban")
	}
	waitForCondition(t, time.Second, conn.closed.Load, "the banned connection must be closed")

	banned := guardIsBanned(g, "10.0.0.1")
	if !banned {
		t.Fatal("the IP must be banned")
	}

	first := guardBanUntil(g, "10.0.0.1")
	if first == 0 {
		t.Fatal("the ban expiration must be recorded")
	}

	// A strike during an active ban must not extend it.
	if g.strike(conn) {
		t.Fatal("a strike during an active ban must not trigger another ban")
	}
	if got := guardBanUntil(g, "10.0.0.1"); got != first {
		t.Fatalf("an active ban must not be extended: first=%d, got=%d", first, got)
	}

	// Expire the ban and strike again to exercise the escalation path.
	guardExpireBan(g, "10.0.0.1")

	if !g.strike(conn) {
		t.Fatal("a strike after the ban expires must trigger another ban")
	}

	second := guardBanUntil(g, "10.0.0.1")
	if second-first < int64(time.Minute) {
		t.Fatalf("the ban duration must escalate: first=%d, second=%d", first, second)
	}
}

func TestGuardBanDoesNotCloseRecycledConn(t *testing.T) {
	g := newGuard(&guardOptions{
		strikeThreshold: 1,
		strikeWindow:    time.Minute,
		minBanDuration:  time.Minute,
		maxBanDuration:  time.Hour,
	})

	first := newRecycledConn(1)
	second := newGuardConn(2, "10.0.0.1")

	if !g.admit(first) || !g.admit(second) {
		t.Fatal("the connections must be admitted")
	}

	// The transport recycles the first object for a newer connection before the ban is applied.
	first.currentID.Store(100)

	if !g.strike(second) {
		t.Fatal("the strike must trigger a ban")
	}

	select {
	case id := <-first.closeIDs:
		if id != 1 {
			t.Fatalf("the captured ID must be used for the close: %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("the recycled connection must receive a guarded close attempt")
	}

	waitForCondition(t, time.Second, second.closed.Load, "the struck connection must be closed")

	if first.closed.Load() {
		t.Fatal("a recycled connection object must not be closed")
	}
	if !second.closed.Load() {
		t.Fatal("the struck connection must be closed")
	}
}

func TestGuardWhitelist(t *testing.T) {
	g := newGuard(&guardOptions{
		maxConns:  1,
		whitelist: []string{"10.0.0.0/8"},
	})

	first := newGuardConn(1, "10.1.2.3")
	second := newGuardConn(2, "10.1.2.3")

	if !g.admit(first) || !g.admit(second) {
		t.Fatal("a whitelisted IP must bypass the connection limit")
	}

	limited := newGuardConn(4, "192.168.1.1")
	other := newGuardConn(5, "192.168.1.1")
	if !g.admit(limited) || g.admit(other) {
		t.Fatal("a non-whitelisted IP must still be limited")
	}
}

func TestGateGuardIPMaxConns(t *testing.T) {
	f := newGateFixture(t)
	f.gate.guard = newGuard(&guardOptions{maxConns: 1, strikeThreshold: 0})
	f.start(t)

	first := newGateConn(4101, 0)
	f.server.connect(first)

	if ok, _ := f.gate.session.has(cluster.Conn, first.ID()); !ok {
		t.Fatal("the first connection must be registered")
	}

	second := newGateConn(4102, 0)
	f.server.connect(second)

	waitForCondition(t, time.Second, second.closed.Load, "the second connection must be closed")
	if ok, _ := f.gate.session.has(cluster.Conn, second.ID()); ok {
		t.Fatal("the rejected connection must not be registered")
	}

	f.server.disconnect(first)
}

func TestGateGuardAutoBan(t *testing.T) {
	f := newGateFixture(t)
	f.gate.limiter = newConnLimiter(&limiterOptions{
		quota: 1, period: time.Hour, burst: 1, closeThreshold: 1,
	})
	f.gate.guard = newGuard(&guardOptions{
		strikeThreshold: 1,
		strikeWindow:    time.Minute,
		minBanDuration:  time.Minute,
		maxBanDuration:  time.Hour,
	})
	f.start(t)

	conn := newGateConn(4201, 0)
	f.server.connect(conn)

	// The first frame fills the bucket; the second one exceeds the limit, closes the connection
	// and strikes the IP, which triggers a ban.
	for range 2 {
		buf, err := packet.PackMessage(1, 1, []byte("payload"))
		if err != nil {
			t.Fatalf("pack message failed: %v", err)
		}
		f.server.receive(conn, buf)
	}

	waitForCondition(t, time.Second, conn.closed.Load, "the limited connection must be closed")
	waitForCondition(t, time.Second, func() bool {
		return guardIsBanned(f.gate.guard, "10.0.0.1")
	}, "the IP must be banned")

	f.server.disconnect(conn)
}

// guardIsBanned reports whether ip carries an active ban in the guard.
func guardIsBanned(g *guard, ip string) bool {
	addr, ok := parseIP(ip)
	if !ok {
		return false
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry := shard.entries[key]

	return entry != nil && g.now() < entry.banUntil
}

// guardExpireBan clears the active ban of ip while keeping its escalation state.
func guardExpireBan(g *guard, ip string) {
	addr, ok := parseIP(ip)
	if !ok {
		return
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	shard.mu.Lock()
	if entry := shard.entries[key]; entry != nil {
		entry.banUntil = 0
		entry.lastSeen = g.now()
	}
	shard.mu.Unlock()
}

// guardBanUntil returns the ban expiration of ip relative to the guard epoch.
func guardBanUntil(g *guard, ip string) int64 {
	addr, ok := parseIP(ip)
	if !ok {
		return 0
	}

	key := ipKey(addr)
	shard := g.loadShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	if entry := shard.entries[key]; entry != nil {
		return entry.banUntil
	}

	return 0
}
