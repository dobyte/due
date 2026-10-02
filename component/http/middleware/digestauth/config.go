package digestauth

import (
	"time"

	"github.com/dobyte/due/component/http/v2"
)

// Config is the Digest authentication middleware configuration.
type Config struct {
	// Next is the predicate that decides whether to skip the middleware; the middleware is
	// skipped when it returns true.
	//
	// Optional. Default: nil
	Next func(ctx http.Context) bool

	// Users is the list of allowed credentials, keyed by username with the precomputed HA1 as
	// value:
	//
	//	HA1 = MD5(username:realm:password)
	//
	// When Users is empty, Authorizer must be set.
	//
	// Optional. Default: map[string]string{}
	Users map[string]string

	// Authorizer is a custom credential verification function. It verifies the credentials by
	// username and reports the matching HA1 string and whether the check passed. It returns
	// "", false when the user does not exist.
	//
	// Optional. Default: nil
	Authorizer func(username string) (ha1 string, ok bool)

	// Unauthorized handles an unauthorized response. By default it returns 401 Unauthorized with
	// a correct WWW-Authenticate header.
	//
	// Optional. Default: nil
	Unauthorized http.Handler

	// BadRequest handles the response for a malformed Authorization header. By default it
	// returns 400 Bad Request without a WWW-Authenticate header.
	//
	// Optional. Default: nil
	BadRequest http.Handler

	// Realm sets the DigestAuth realm attribute that identifies the authentication system.
	//
	// Optional. Default: "Restricted"
	Realm string

	// HeaderLimit is the maximum allowed length of the Authorization header. Requests exceeding
	// it are rejected.
	//
	// Optional. Default: 8192
	HeaderLimit int

	// NonceTTL is the lifetime of a nonce value. Within this window every request must carry a
	// monotonically increasing nonce count (nc), otherwise it is rejected as a replay.
	//
	// Optional. Default: 5 * time.Minute
	NonceTTL time.Duration

	// ContextUsernameKey is the context key under which the username is stored.
	//
	// Optional. Default: "username"
	ContextUsernameKey string
}

// ConfigDefault is the default configuration.
var ConfigDefault = Config{
	Next:               nil,
	Users:              map[string]string{},
	Realm:              "Restricted",
	HeaderLimit:        8192,
	NonceTTL:           5 * time.Minute,
	Authorizer:         nil,
	Unauthorized:       nil,
	BadRequest:         nil,
	ContextUsernameKey: "username",
}

// configDefault returns config with the unset fields filled in from [ConfigDefault].
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if cfg.Users == nil {
		cfg.Users = make(map[string]string)
	}

	if cfg.Realm == "" {
		cfg.Realm = ConfigDefault.Realm
	}

	if cfg.HeaderLimit <= 0 {
		cfg.HeaderLimit = ConfigDefault.HeaderLimit
	}

	if cfg.NonceTTL <= 0 {
		cfg.NonceTTL = ConfigDefault.NonceTTL
	}

	if cfg.ContextUsernameKey == "" {
		cfg.ContextUsernameKey = ConfigDefault.ContextUsernameKey
	}

	return cfg
}
