package digestauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/component/http/v2"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/utils/xhash"
	"github.com/gofiber/fiber/v3"
)

const (
	digestScheme      = "Digest"
	nonceMaxCount     = 10000            // Maximum number of cached nonces, preventing unbounded memory growth from unauthorized requests
	nonceCleanupEvery = 10 * time.Minute // Minimum interval between expired-nonce cleanups
)

// New returns an HTTP Digest authentication middleware based on RFC 2617.
//
// Usage: group := hp.Router().Group("/path", middleware.Digest(middleware.DigestConfig{...}))
func New(config ...Config) http.Handler {
	da := newDigestAuth(config...)

	return func(ctx http.Context) error {
		// Don't execute middleware if Next returns true
		if da.config.Next != nil && da.config.Next(ctx) {
			return ctx.Next()
		}

		// Get authorization header
		rawAuth := ctx.Get(fiber.HeaderAuthorization)
		if rawAuth == "" {
			return da.unauthorized(ctx)
		}

		// Check header length limit
		if len(rawAuth) > da.config.HeaderLimit {
			return da.badRequest(ctx)
		}

		// Check for invalid characters
		if containsInvalidHeaderChars(rawAuth) {
			return da.badRequest(ctx)
		}

		// Verify Digest scheme
		auth := strings.TrimSpace(rawAuth)
		if len(auth) < len(digestScheme) || !strings.EqualFold(auth[:len(digestScheme)], digestScheme) {
			return da.badRequest(ctx)
		}

		rest := auth[len(digestScheme):]
		if len(rest) < 2 || rest[0] != ' ' || rest[1] == ' ' {
			return da.badRequest(ctx)
		}

		// Parse digest parameters
		params := parseDigestParams(rest[1:])
		if len(params) == 0 {
			return da.badRequest(ctx)
		}

		var (
			username = params["username"]
			realm    = params["realm"]
			nonce    = params["nonce"]
			uri      = params["uri"]
			response = params["response"]
			qop      = params["qop"]
			nc       = params["nc"]
			cnonce   = params["cnonce"]
		)

		// Validate required fields
		if username == "" || nonce == "" || uri == "" || response == "" {
			return da.badRequest(ctx)
		}

		// Validate realm
		if realm != da.config.Realm {
			return da.unauthorized(ctx)
		}

		// Validate nonce
		if !da.validateNonce(nonce) {
			return da.unauthorized(ctx)
		}

		// Lookup user's HA1
		// HA1 = MD5(username:realm:password), provided directly by the configuration.
		ha1, ok := da.config.Users[username]
		if !ok {
			if da.config.Authorizer != nil {
				ha1, ok = da.config.Authorizer(username)
			}
		}

		if !ok {
			return da.unauthorized(ctx)
		}

		// HA2 = MD5(method:uri)
		ha2 := xhash.MD5(ctx.Method() + ":" + uri)

		// The server only advertises qop="auth" and rejects requests with a missing or different qop,
		// avoiding a fallback to the legacy digest authentication without replay protection.
		if qop != "auth" {
			return da.badRequest(ctx)
		}

		// nc and cnonce are required when qop is used.
		if nc == "" || cnonce == "" {
			return da.badRequest(ctx)
		}

		// nc must be an 8-digit hexadecimal count (RFC 2617).
		if len(nc) != 8 {
			return da.badRequest(ctx)
		}

		nonceCount, err := strconv.ParseUint(nc, 16, 64)
		if err != nil {
			return da.badRequest(ctx)
		}

		// Compute the expected response.
		expected := xhash.MD5(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)

		// Compare the response in constant time to prevent timing side channels; hex values are
		// case-insensitive, so normalize the received response to lower case before comparing.
		if subtle.ConstantTimeCompare([]byte(strings.ToLower(response)), []byte(expected)) != 1 {
			return da.unauthorized(ctx)
		}

		// Consume the nonce count only after the response check passes, preventing replays.
		if !da.consumeNonce(nonce, nonceCount) {
			return da.unauthorized(ctx)
		}

		// Store username in context
		ctx.Locals(da.config.ContextUsernameKey, username)

		return ctx.Next()
	}
}

// containsInvalidHeaderChars reports whether s holds any byte outside the
// valid header set: HTAB ('\t') or visible ASCII [0x20, 0x7E].
func containsInvalidHeaderChars(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 0x20 && c != '\t') || c >= 0x7f {
			return true
		}
	}

	return false
}

// parseDigestParams parses a Digest authentication parameter string into a map.
//
// It follows the RFC 2617 syntax and correctly handles quoted values that contain commas or
// escaped quotes.
func parseDigestParams(s string) map[string]string {
	params := make(map[string]string)
	for _, part := range splitDigestParams(s) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = strings.ReplaceAll(val[1:len(val)-1], `\"`, `"`)
		}
		params[key] = val
	}
	return params
}

// splitDigestParams splits a Digest parameter string on commas, ignoring commas inside quotes.
func splitDigestParams(s string) []string {
	var (
		parts   []string
		start   int
		inQuote bool
		escaped bool
	)

	for i := 0; i < len(s); i++ {
		switch {
		case escaped:
			escaped = false
		case s[i] == '\\' && inQuote:
			escaped = true
		case s[i] == '"':
			inQuote = !inQuote
		case s[i] == ',' && !inQuote:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}

	return append(parts, s[start:])
}

// nonceEntry is a nonce cache entry.
type nonceEntry struct {
	createdAt time.Time // Creation time
	nc        uint64    // Highest consumed nonce count
}

type digestAuth struct {
	mu          sync.Mutex
	nonces      map[string]*nonceEntry
	nonceOrder  []string  // Records the nonce creation order, used to evict the oldest entries when the capacity is exceeded
	lastCleanup time.Time // Time of the last expired-nonce cleanup
	config      Config
}

func newDigestAuth(config ...Config) *digestAuth {
	return &digestAuth{
		nonces: make(map[string]*nonceEntry),
		config: configDefault(config...),
	}
}

// unauthorized writes an unauthorized response.
//
// It returns a 401 response with a WWW-Authenticate header.
func (da *digestAuth) unauthorized(ctx http.Context) error {
	if da.config.Unauthorized != nil {
		return da.config.Unauthorized(ctx)
	} else {
		nonce := da.generateNonce()

		ctx.Set("WWW-Authenticate",
			fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5, qop="auth"`, da.config.Realm, nonce))

		return ctx.Status(fiber.StatusUnauthorized).JSON(&http.Resp{
			Code:    codes.Unauthorized.Code(),
			Message: codes.Unauthorized.Message(),
		})
	}
}

// badRequest writes a bad request response.
func (da *digestAuth) badRequest(ctx http.Context) error {
	if da.config.BadRequest != nil {
		return da.config.BadRequest(ctx)
	} else {
		return ctx.Status(fiber.StatusBadRequest).JSON(&http.Resp{
			Code:    codes.IllegalRequest.Code(),
			Message: codes.IllegalRequest.Message(),
		})
	}
}

// generateNonce generates and caches a nonce.
//
// It cleans up expired entries on demand and evicts the oldest nonces when the capacity is
// exceeded.
func (s *digestAuth) generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := base64.RawStdEncoding.EncodeToString(b)

	s.mu.Lock()

	now := time.Now()

	// Clean up expired nonces on demand to avoid a dedicated cleanup goroutine.
	if s.lastCleanup.IsZero() || now.Sub(s.lastCleanup) >= nonceCleanupEvery {
		order := s.nonceOrder[:0]
		for _, n := range s.nonceOrder {
			entry, ok := s.nonces[n]
			if !ok {
				continue
			}

			if now.Sub(entry.createdAt) > s.config.NonceTTL {
				delete(s.nonces, n)
				continue
			}

			order = append(order, n)
		}
		s.nonceOrder = order
		s.lastCleanup = now
	}

	// Evict the oldest nonces when the capacity is exceeded, preventing unbounded memory growth
	// from unauthorized requests.
	for len(s.nonces) >= nonceMaxCount && len(s.nonceOrder) > 0 {
		delete(s.nonces, s.nonceOrder[0])
		s.nonceOrder = s.nonceOrder[1:]
	}

	s.nonces[nonce] = &nonceEntry{createdAt: now}
	s.nonceOrder = append(s.nonceOrder, nonce)
	s.mu.Unlock()

	return nonce
}

// validateNonce reports whether nonce exists and has not expired.
func (s *digestAuth) validateNonce(nonce string) bool {
	s.mu.Lock()

	entry, ok := s.nonces[nonce]
	if !ok {
		s.mu.Unlock()
		return false
	}

	valid := time.Since(entry.createdAt) <= s.config.NonceTTL
	s.mu.Unlock()

	return valid
}

// consumeNonce consumes the nonce count.
//
// It requires nc to increase monotonically in order to prevent replays. It reports whether the
// count was valid and has been updated; it returns false when the nonce is invalid or the count
// did not increase.
func (s *digestAuth) consumeNonce(nonce string, nc uint64) bool {
	s.mu.Lock()

	entry, ok := s.nonces[nonce]
	if !ok {
		s.mu.Unlock()
		return false
	}

	if nc <= entry.nc {
		s.mu.Unlock()
		return false
	}

	entry.nc = nc
	s.mu.Unlock()

	return true
}
