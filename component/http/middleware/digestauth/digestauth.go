package digestauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/component/http/v2"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/utils/xhash"
	"github.com/gofiber/fiber/v3"
)

const digestScheme = "Digest"

// New 返回一个基于RFC 2617的HTTP Digest认证中间件
// 用法: group := hp.Router().Group("/path", middleware.Digest(middleware.DigestConfig{...}))
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
		password, ok := da.config.Users[username]
		if !ok {
			if da.config.Authorizer != nil {
				password, ok = da.config.Authorizer(username)
			}
		}

		if !ok {
			return da.unauthorized(ctx)
		}

		// HA1 = MD5(username:realm:password)
		ha1 := xhash.MD5(username + ":" + realm + ":" + password)

		// HA2 = MD5(method:uri)
		ha2 := xhash.MD5(ctx.Method() + ":" + uri)

		// 计算期望的response
		var expected string

		if qop == "auth" || qop == "auth-int" {
			expected = xhash.MD5(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
		} else {
			expected = xhash.MD5(ha1 + ":" + nonce + ":" + ha2)
		}

		if response != expected {
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

// parseDigestParams 解析Digest认证参数字符串为map
func parseDigestParams(s string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(kv[1], `"`)
		params[key] = val
	}
	return params
}

type digestAuth struct {
	mu     sync.RWMutex
	nonces map[string]time.Time
	config Config
}

func newDigestAuth(config ...Config) *digestAuth {
	da := &digestAuth{
		nonces: make(map[string]time.Time),
		config: configDefault(config...),
	}

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			da.cleanup()
		}
	}()

	return da
}

// 未授权处理函数
func (da *digestAuth) unauthorized(ctx http.Context) error {
	if da.config.Unauthorized != nil {
		return da.config.Unauthorized(ctx)
	} else {
		var (
			nonce  = da.generateNonce()
			opaque = da.opaqueNonce()
		)

		ctx.Set("WWW-Authenticate",
			fmt.Sprintf(`Digest realm="%s", nonce="%s", opaque="%s", algorithm=MD5, qop="auth"`, da.config.Realm, nonce, opaque))

		return ctx.Status(fiber.StatusUnauthorized).JSON(&http.Resp{
			Code:    codes.Unauthorized.Code(),
			Message: codes.Unauthorized.Message(),
		})
	}
}

// 错误请求
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

func (s *digestAuth) generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := base64.RawStdEncoding.EncodeToString(b)
	s.mu.Lock()
	s.nonces[nonce] = time.Now()
	s.mu.Unlock()
	return nonce
}

func (s *digestAuth) opaqueNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return base64.RawStdEncoding.EncodeToString(b)
}

func (s *digestAuth) validateNonce(nonce string) bool {
	s.mu.RLock()
	createdAt, ok := s.nonces[nonce]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return time.Since(createdAt) <= s.config.NonceTTL
}

func (s *digestAuth) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for nonce, createdAt := range s.nonces {
		if now.Sub(createdAt) > s.config.NonceTTL {
			delete(s.nonces, nonce)
		}
	}
}
