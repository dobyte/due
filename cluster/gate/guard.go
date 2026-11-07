package gate

import (
	"context"
	"hash/maphash"
	"math"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	// defaultGuardShardNum is the number of per-IP guard table shards (a power of two, so that
	// the shard is selected with a bit operation).
	defaultGuardShardNum = 256

	// defaultGuardSweepInterval is the interval of the expired entry cleanup.
	defaultGuardSweepInterval = time.Minute

	// defaultGuardReportInterval is the interval between two aggregated rejection reports.
	defaultGuardReportInterval = time.Second

	// defaultGuardEntryTTL is how long an idle entry without connections and bans is retained.
	defaultGuardEntryTTL = 10 * time.Minute

	// defaultGuardMaxEntries caps the number of retained IP entries. When the table is full, new
	// IPs are not tracked so that memory stays bounded under a distributed attack.
	defaultGuardMaxEntries = 1 << 16
)

// ipHashSeed hashes normalized IP keys to select a guard shard.
var ipHashSeed = maphash.MakeSeed()

// guardShard is a shard of the per-IP admission and ban table.
//
// Unlike the per-connection limiter, the guard is only touched when a connection is admitted,
// removed, banned or struck, so a plain mutex is enough.
type guardShard struct {
	mu      sync.Mutex
	entries map[netip.Prefix]*ipEntry
	_       [48]byte // pads the shard to a cache line
}

// ipEntry holds the admission, violation and ban state of one IP.
//
// All fields are protected by the shard mutex.
type ipEntry struct {
	conns       map[int64]network.Conn // admitted connections of the IP
	acceptTat   int64                  // next allowed admission time, relative to the guard epoch
	strikes     int                    // violations in the current window
	windowStart int64                  // start of the current strike window
	banUntil    int64                  // ban expiration, relative to the guard epoch
	banCount    int                    // bans issued recently, used for escalation
	lastBanAt   int64                  // time of the last ban, relative to the guard epoch
	lastSeen    int64                  // last admission, removal or strike, relative to the epoch
}

// connRef pairs an admitted connection with the ID it had when it was captured.
//
// Server connection objects are recycled and reused, so a delayed close must compare the captured
// ID with [network.Conn.CloseIf] instead of reading the current connection ID.
type connRef struct {
	id   int64
	conn network.Conn
}

// newConnRef pairs conn with the ID captured for this connection generation. The ID must be read
// while conn still represents the tracked connection, or taken from the admission-time entry key.
func newConnRef(id int64, conn network.Conn) connRef {
	return connRef{id: id, conn: conn}
}

// strikeResult describes the connections to close and the ban expiration after a violation.
type strikeResult struct {
	conns []connRef
	until int64
}

// guardOptions are the per-IP admission and ban options.
type guardOptions struct {
	maxConns        int           // maximum concurrent connections per IP; zero disables the limit
	quota           float64       // admissions allowed per accept period; zero disables the limit
	period          time.Duration // accept quota period
	burst           int           // accept bucket capacity; defaults to ceil(quota)
	whitelist       []string      // IP prefixes that bypass admission and bans
	strikeThreshold int           // violations within a window before banning; zero disables bans
	strikeWindow    time.Duration // window of the violations that lead to a ban
	minBanDuration  time.Duration // first ban duration
	maxBanDuration  time.Duration // maximum ban duration
	maxEntries      int           // maximum retained IP entries; defaults to defaultGuardMaxEntries
}

// normalize validates the guard options and fills in the defaults.
func (o *guardOptions) normalize() {
	if o.period <= 0 {
		o.period = xconv.Duration(defaultGuardPeriod)
	}

	if math.IsNaN(o.quota) || math.IsInf(o.quota, 0) {
		o.quota = 0
	}

	if o.quota > 0 {
		if o.burst <= 0 {
			o.burst = int(math.Ceil(o.quota))
		}

		if o.burst < 1 {
			o.burst = 1
		}
	} else {
		o.quota = 0
		o.burst = 0
	}

	if o.maxConns < 0 {
		o.maxConns = 0
	}

	if o.strikeThreshold < 0 {
		o.strikeThreshold = 0
	}

	if o.strikeWindow <= 0 {
		o.strikeWindow = xconv.Duration(defaultGuardStrikeWindow)
	}

	if o.minBanDuration <= 0 {
		o.minBanDuration = xconv.Duration(defaultGuardMinBanDuration)
	}

	if o.maxBanDuration <= 0 {
		o.maxBanDuration = xconv.Duration(defaultGuardMaxBanDuration)
	}

	if o.maxBanDuration < o.minBanDuration {
		o.maxBanDuration = o.minBanDuration
	}

	if o.maxEntries <= 0 {
		o.maxEntries = defaultGuardMaxEntries
	}
}

// guard enforces per-IP connection admission and bans.
//
// A connection is admitted before it is registered in the session, so a flood is rejected before
// it consumes session and limiter resources. Bans are local to the gate instance and are keyed by
// the normalized peer IP: IPv4-mapped addresses are unmapped and IPv6 addresses are aggregated by
// /64 so that a single client cannot rotate through its own address space.
type guard struct {
	epoch      time.Time // reference time for entry timestamps
	interval   int64     // admission emission interval in nanoseconds
	tolerance  int64     // admission burst tolerance in nanoseconds
	enabled    bool      // whether any guard policy is active
	trackConns bool      // whether per-connection state is needed for limits or bans
	opts       guardOptions
	whitelist  []netip.Prefix
	shards     [defaultGuardShardNum]guardShard

	maxEntries int64        // maximum number of retained IP entries
	_          [56]byte     // keeps entries on a separate cache line
	entries    atomic.Int64 // number of retained IP entries

	_             [56]byte     // keeps rejectedBan on a separate cache line
	rejectedBan   atomic.Int64 // connections rejected because the IP is banned
	_             [56]byte     // keeps rejectedLimit on a separate cache line
	rejectedLimit atomic.Int64 // connections rejected by the IP connection limit
	_             [56]byte     // keeps rejectedRate on a separate cache line
	rejectedRate  atomic.Int64 // connections rejected by the IP admission rate
	_             [56]byte     // keeps untracked on a separate cache line
	untracked     atomic.Int64 // events skipped because the entry table is full

	wg sync.WaitGroup
}

// newGuard creates a per-IP guard with normalized options. The given options are copied so that
// the guard owns its configuration and later option changes cannot race with admission.
func newGuard(opts *guardOptions) *guard {
	if opts == nil {
		opts = &guardOptions{}
	}

	cfg := *opts
	cfg.normalize()

	g := &guard{
		epoch:      time.Now(),
		opts:       cfg,
		maxEntries: int64(cfg.maxEntries),
		trackConns: cfg.maxConns > 0 || cfg.strikeThreshold > 0,
	}
	g.setRateLimited(true)

	if cfg.quota > 0 {
		lo := &limiterOptions{quota: cfg.quota, period: cfg.period}
		g.interval = limiterInterval(lo)
		g.tolerance = limiterTolerance(g.interval, cfg.burst)
	}

	for i := range g.shards {
		g.shards[i].entries = make(map[netip.Prefix]*ipEntry)
	}

	for _, raw := range cfg.whitelist {
		prefix, ok := parseIPPrefix(raw)
		if !ok {
			log.Warnf("invalid IP whitelist entry: %s", raw)
			continue
		}
		g.whitelist = append(g.whitelist, prefix)
	}

	g.opts.whitelist = nil

	return g
}

// setRateLimited recomputes whether the guard is active. A strike threshold only matters when
// violations can come from the connection limiter or from the IP admission limits. It must be
// called before the guard starts.
func (g *guard) setRateLimited(rateLimited bool) {
	g.enabled = g.opts.maxConns > 0 || g.opts.quota > 0 ||
		(g.opts.strikeThreshold > 0 && rateLimited)
}

// start launches the expired entry cleanup and the aggregated rejection report until ctx is
// canceled.
func (g *guard) start(ctx context.Context) {
	if !g.enabled {
		return
	}

	g.wg.Go(func() { g.sweepLoop(ctx) })
	g.wg.Go(func() { g.reportLoop(ctx) })
}

// stop stops the background goroutines and flushes the remaining counters.
func (g *guard) stop() {
	g.wg.Wait()
	g.flush()
}

// sweepLoop removes expired entries until ctx is canceled.
func (g *guard) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(defaultGuardSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.sweep()
		}
	}
}

// reportLoop flushes the aggregated rejection counters until ctx is canceled.
func (g *guard) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(defaultGuardReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.flush()
		}
	}
}

// flush reports the rejections and skipped events since the previous flush and resets the
// counters.
func (g *guard) flush() {
	var (
		banned = g.rejectedBan.Swap(0)
		limit  = g.rejectedLimit.Swap(0)
		rate   = g.rejectedRate.Swap(0)
	)

	if total := banned + limit + rate; total > 0 {
		log.Warnf("connection guard rejected %d connections in the last interval "+
			"(banned: %d, limit: %d, rate: %d)", total, banned, limit, rate)
	}

	if untracked := g.untracked.Swap(0); untracked > 0 {
		log.Warnf("connection guard skipped %d events because the IP entry table is full (max: %d)",
			untracked, g.maxEntries)
	}
}

// sweep removes entries that have no connection, no active ban and no recent activity. An entry
// with recent ban history is retained until the escalation window expires, so a repeat offender
// keeps its escalation level for maxBanDuration after the last ban.
func (g *guard) sweep() {
	now := g.now()

	for i := range g.shards {
		shard := &g.shards[i]
		shard.mu.Lock()
		for key, entry := range shard.entries {
			if len(entry.conns) == 0 && now >= entry.banUntil &&
				now-entry.lastSeen > int64(defaultGuardEntryTTL) &&
				(entry.banCount == 0 || now-entry.lastBanAt > int64(g.opts.maxBanDuration)) {
				delete(shard.entries, key)
				g.entries.Add(-1)
			}
		}
		shard.mu.Unlock()
	}
}

// admit reports whether conn may be registered. A rejected connection is closed asynchronously,
// so the caller must not register it in the session.
func (g *guard) admit(conn network.Conn) bool {
	if !g.enabled {
		return true
	}

	addr, ok := remoteIP(conn)
	if !ok || g.whitelisted(addr) {
		return true
	}

	var (
		cid   = conn.ID()
		key   = ipKey(addr)
		shard = g.loadShard(key)
		now   = g.now()
	)

	shard.mu.Lock()

	entry, tracked := g.entryLocked(shard, key)
	if !tracked {
		shard.mu.Unlock()
		return true
	}
	entry.lastSeen = now

	if now < entry.banUntil {
		shard.mu.Unlock()
		g.rejectedBan.Add(1)
		closeConns([]connRef{newConnRef(cid, conn)})
		return false
	}

	if g.opts.maxConns > 0 && len(entry.conns) >= g.opts.maxConns {
		result, banned := g.strikeLocked(entry, now)
		conns := append(result.conns, newConnRef(cid, conn))
		shard.mu.Unlock()
		if banned {
			g.logBan(key, result.until, len(conns))
		}
		g.rejectedLimit.Add(1)
		closeConns(conns)
		return false
	}

	if g.opts.quota > 0 && !g.allowAcceptLocked(entry, now) {
		result, banned := g.strikeLocked(entry, now)
		conns := append(result.conns, newConnRef(cid, conn))
		shard.mu.Unlock()
		if banned {
			g.logBan(key, result.until, len(conns))
		}
		g.rejectedRate.Add(1)
		closeConns(conns)
		return false
	}

	if g.trackConns {
		entry.conns[cid] = conn
	}
	shard.mu.Unlock()

	return true
}

// remove releases the connection of conn from its IP entry.
//
// The connection ID is read from conn, so the transport must invoke the disconnect callback before
// recycling the object; a recycled object may already carry the ID and address of a newer
// connection.
func (g *guard) remove(conn network.Conn) {
	if !g.enabled {
		return
	}

	addr, ok := remoteIP(conn)
	if !ok || g.whitelisted(addr) {
		return
	}

	var (
		key   = ipKey(addr)
		shard = g.loadShard(key)
		now   = g.now()
	)

	shard.mu.Lock()
	if entry := shard.entries[key]; entry != nil {
		if g.trackConns {
			delete(entry.conns, conn.ID())
		}
		entry.lastSeen = now
	}
	shard.mu.Unlock()
}

// strike records a violation of conn and reports whether it triggered a ban. When a ban is
// triggered the related connections, including conn, are closed asynchronously.
func (g *guard) strike(conn network.Conn) bool {
	if !g.enabled || g.opts.strikeThreshold <= 0 {
		return false
	}

	addr, ok := remoteIP(conn)
	if !ok || g.whitelisted(addr) {
		return false
	}

	var (
		cid   = conn.ID()
		key   = ipKey(addr)
		shard = g.loadShard(key)
		now   = g.now()
	)

	shard.mu.Lock()

	entry, tracked := g.entryLocked(shard, key)
	if !tracked {
		shard.mu.Unlock()
		return false
	}
	entry.lastSeen = now

	// A ban already in effect is not extended by further violations from the same connection; the
	// caller still closes the offending connection.
	if now < entry.banUntil {
		shard.mu.Unlock()
		return false
	}

	result, banned := g.strikeLocked(entry, now)
	if !banned {
		shard.mu.Unlock()
		return false
	}

	conns := ensureConn(result.conns, newConnRef(cid, conn))
	shard.mu.Unlock()

	g.logBan(key, result.until, len(conns))
	closeConns(conns)

	return true
}

// entryLocked returns the entry of key, creating it when the table has room. The caller must hold
// the shard lock.
func (g *guard) entryLocked(shard *guardShard, key netip.Prefix) (*ipEntry, bool) {
	if entry := shard.entries[key]; entry != nil {
		return entry, true
	}

	if g.entries.Add(1) > g.maxEntries {
		g.entries.Add(-1)
		g.untracked.Add(1)
		return nil, false
	}

	var conns map[int64]network.Conn
	if g.trackConns {
		conns = make(map[int64]network.Conn)
	}

	entry := &ipEntry{conns: conns}
	shard.entries[key] = entry

	return entry, true
}

// logBan reports a ban outside the shard lock.
func (g *guard) logBan(key netip.Prefix, until int64, connections int) {
	ip := key.Addr().String()
	if !key.Addr().Is4() {
		ip = key.String()
	}

	log.Warnf("ip banned, ip: %s, until: %s, connections: %d", ip,
		g.epoch.Add(time.Duration(until)).Format(time.RFC3339), connections)
}

// strikeLocked records one violation and reports whether it triggered a ban together with the
// connections to close. The caller must hold the shard lock.
func (g *guard) strikeLocked(entry *ipEntry, now int64) (strikeResult, bool) {
	if g.opts.strikeThreshold <= 0 {
		return strikeResult{}, false
	}

	if now-entry.windowStart > int64(g.opts.strikeWindow) {
		entry.strikes = 0
		entry.windowStart = now
	}

	if entry.strikes < math.MaxInt32 {
		entry.strikes++
	}

	if entry.strikes < g.opts.strikeThreshold {
		return strikeResult{}, false
	}

	return g.banLocked(entry, now), true
}

// banLocked escalates the ban duration and returns the connections to close and the expiration.
// The caller must hold the shard lock and logs the ban after releasing it.
func (g *guard) banLocked(entry *ipEntry, now int64) strikeResult {
	if entry.banCount > 0 && now-entry.lastBanAt > int64(g.opts.maxBanDuration) {
		entry.banCount = 0
	}

	if entry.banCount < 64 {
		entry.banCount++
	}

	duration := g.opts.minBanDuration
	for n := entry.banCount - 1; n > 0; n-- {
		if duration > g.opts.maxBanDuration/2 {
			duration = g.opts.maxBanDuration
			break
		}
		duration *= 2
	}
	if duration > g.opts.maxBanDuration {
		duration = g.opts.maxBanDuration
	}

	entry.banUntil = now + int64(duration)
	entry.lastBanAt = now
	entry.strikes = 0
	entry.windowStart = now

	conns := make([]connRef, 0, len(entry.conns))
	for id, conn := range entry.conns {
		// Use the entry key captured at admission; reading conn.ID() here could observe a
		// recycled object that already belongs to a newer connection.
		conns = append(conns, newConnRef(id, conn))
	}
	clear(entry.conns)

	return strikeResult{conns: conns, until: entry.banUntil}
}

// allowAcceptLocked applies the admission GCRA of entry and reports whether the admission is
// allowed. The caller must hold the shard lock.
func (g *guard) allowAcceptLocked(entry *ipEntry, now int64) bool {
	switch {
	case now >= entry.acceptTat:
		entry.acceptTat = now + g.interval
		return true
	case entry.acceptTat-now <= g.tolerance:
		entry.acceptTat += g.interval
		return true
	default:
		return false
	}
}

// whitelisted reports whether addr matches the configured whitelist.
func (g *guard) whitelisted(addr netip.Addr) bool {
	for _, prefix := range g.whitelist {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// loadShard returns the shard of the given IP key.
func (g *guard) loadShard(key netip.Prefix) *guardShard {
	addr := key.Addr().As16()

	return &g.shards[(maphash.Bytes(ipHashSeed, addr[:])^uint64(key.Bits()))&(defaultGuardShardNum-1)]
}

// now returns the current time in nanoseconds relative to the guard epoch.
func (g *guard) now() int64 {
	return int64(time.Since(g.epoch))
}

// remoteIP returns the normalized peer address of conn.
func remoteIP(conn network.Conn) (netip.Addr, bool) {
	ip, err := conn.RemoteIP()
	if err != nil || ip == "" {
		return netip.Addr{}, false
	}

	return parseIP(ip)
}

// parseIP parses raw into an unmapped address without a zone.
func parseIP(raw string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}

	return addr.Unmap().WithZone(""), true
}

// ipKey returns the normalized key of addr. IPv4 addresses use a /32 prefix and IPv6 addresses
// are aggregated by /64. The prefix is comparable and can be used as a map key without allocating
// a string on every admission or removal.
func ipKey(addr netip.Addr) netip.Prefix {
	if addr.Is6() && !addr.Is4() {
		return netip.PrefixFrom(addr, 64).Masked()
	}

	return netip.PrefixFrom(addr, 32)
}

// parseIPPrefix parses raw as a CIDR prefix, or as a single address with a host prefix.
func parseIPPrefix(raw string) (netip.Prefix, bool) {
	if prefix, err := netip.ParsePrefix(raw); err == nil {
		return prefix.Masked(), true
	}

	addr, ok := parseIP(raw)
	if !ok {
		return netip.Prefix{}, false
	}

	if addr.Is4() {
		return netip.PrefixFrom(addr, 32), true
	}

	return netip.PrefixFrom(addr, 128), true
}

// ensureConn appends ref to refs when its connection ID is not already present.
func ensureConn(refs []connRef, ref connRef) []connRef {
	for _, item := range refs {
		if item.id == ref.id {
			return refs
		}
	}

	return append(refs, ref)
}

// closeConns closes refs asynchronously so that a caller running inside a transport read loop does
// not wait for its own callback to return. The captured ID is compared before closing, so a
// recycled connection object that now carries a newer connection is not closed by mistake.
func closeConns(refs []connRef) {
	if len(refs) == 0 {
		return
	}

	xcall.Go(func() {
		for _, ref := range refs {
			if err := ref.conn.CloseIf(ref.id); err != nil &&
				!errors.Is(err, errors.ErrConnectionClosed) &&
				!errors.Is(err, errors.ErrConnectionNotOpened) {
				log.Warnf("close conn failed, cid: %d, err: %v", ref.id, err)
			}
		}
	})
}
