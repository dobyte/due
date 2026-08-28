package digestauth

import (
	"crypto/rand"
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
	nonceMaxCount     = 10000            // nonce 最大缓存数量，防止未授权请求导致内存无限增长
	nonceCleanupEvery = 10 * time.Minute // nonce 过期清理的最小间隔
)

// New 返回一个基于RFC 2617的HTTP Digest认证中间件
// 用法: group := hp.Router().Group("/path", middleware.Digest(middleware.DigestConfig{...}))
// @param config ...Config 中间件配置
// @return @1 http.Handler 认证中间件
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
		// HA1 = MD5(username:realm:password)，由配置项直接提供
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

		// 计算期望的response
		var expected string
		withQop := qop == "auth" || qop == "auth-int"

		if withQop {
			// 使用qop时必须携带nc与cnonce
			if nc == "" || cnonce == "" {
				return da.badRequest(ctx)
			}

			expected = xhash.MD5(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
		} else {
			expected = xhash.MD5(ha1 + ":" + nonce + ":" + ha2)
		}

		if response != expected {
			return da.unauthorized(ctx)
		}

		// 校验response通过后消费nonce计数，防止重放
		if withQop {
			// nc必须为8位十六进制计数（RFC 2617）
			nonceCount, err := strconv.ParseUint(nc, 16, 64)
			if err != nil {
				return da.badRequest(ctx)
			}

			if !da.consumeNonce(nonce, nonceCount) {
				return da.unauthorized(ctx)
			}
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

// nonceEntry nonce缓存条目
type nonceEntry struct {
	createdAt time.Time // 创建时间
	nc        uint64    // 已使用的最大nonce计数
}

type digestAuth struct {
	mu          sync.Mutex
	nonces      map[string]*nonceEntry
	nonceOrder  []string  // 记录nonce的生成顺序，用于容量超限时淘汰最旧条目
	lastCleanup time.Time // 上次过期清理时间
	config      Config
}

func newDigestAuth(config ...Config) *digestAuth {
	return &digestAuth{
		nonces: make(map[string]*nonceEntry),
		config: configDefault(config...),
	}
}

// 未授权处理函数
// 返回401响应并携带WWW-Authenticate头
// @param ctx http.Context HTTP上下文
// @return @1 error 处理失败时返回的错误
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

// 生成并缓存nonce
// 内部按需清理过期条目，并在容量超限时淘汰最旧的nonce
func (s *digestAuth) generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	nonce := base64.RawStdEncoding.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 按需清理过期nonce，避免常驻清理协程
	if s.lastCleanup.IsZero() || now.Sub(s.lastCleanup) >= nonceCleanupEvery {
		for n, entry := range s.nonces {
			if now.Sub(entry.createdAt) > s.config.NonceTTL {
				delete(s.nonces, n)
			}
		}
		s.lastCleanup = now
	}

	// 容量超限时淘汰最旧的nonce，防止未授权请求导致内存无限增长
	for len(s.nonces) >= nonceMaxCount && len(s.nonceOrder) > 0 {
		delete(s.nonces, s.nonceOrder[0])
		s.nonceOrder = s.nonceOrder[1:]
	}

	s.nonces[nonce] = &nonceEntry{createdAt: now}
	s.nonceOrder = append(s.nonceOrder, nonce)

	return nonce
}

func (s *digestAuth) opaqueNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return base64.RawStdEncoding.EncodeToString(b)
}

// 校验nonce是否存在且未过期
// @param nonce string 待校验的nonce值
// @return @1 bool nonce有效返回true
func (s *digestAuth) validateNonce(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.nonces[nonce]
	if !ok {
		return false
	}

	return time.Since(entry.createdAt) <= s.config.NonceTTL
}

// 消费nonce计数，要求nc单调递增，防止重放
// @return @1 bool 计数有效并已更新返回true；nonce无效或计数未递增返回false
func (s *digestAuth) consumeNonce(nonce string, nc uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.nonces[nonce]
	if !ok {
		return false
	}

	if nc <= entry.nc {
		return false
	}

	entry.nc = nc

	return true
}
