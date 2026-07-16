package digestauth

import (
	"time"

	"github.com/dobyte/due/component/http/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(ctx http.Context) bool

	// Users defines the allowed credentials.
	// The key is the username, the value is the pre-computed HA1:
	// HA1 = MD5(username:realm:password)
	//
	// If Users is empty, Authorizer must be set.
	//
	// Optional. Default: map[string]string{}
	Users map[string]string

	// Authorizer defines a function you can pass
	// to check the credentials however you want.
	// It will be called with a username and is expected to return
	// the corresponding HA1 string and true to indicate the
	// credentials were approved.
	// If the user does not exist, return "", false.
	//
	// Optional. Default: nil
	Authorizer func(username string) (ha1 string, ok bool)

	// Unauthorized defines the response body for unauthorized responses.
	// By default it will return with a 401 Unauthorized and the correct
	// WWW-Authenticate header.
	//
	// Optional. Default: nil
	Unauthorized http.Handler

	// BadRequest defines the response body for malformed Authorization headers.
	// By default it will return with a 400 Bad Request without the
	// WWW-Authenticate header.
	//
	// Optional. Default: nil
	BadRequest http.Handler

	// Realm is a string to define realm attribute of DigestAuth.
	// The realm identifies the system to authenticate against.
	//
	// Optional. Default: "Restricted".
	Realm string

	// HeaderLimit specifies the maximum allowed length of the
	// Authorization header. Requests exceeding this limit will
	// be rejected.
	//
	// Optional. Default: 8192.
	HeaderLimit int

	// NonceTTL specifies the time-to-live for nonce values.
	// Each nonce can only be used once within this time window.
	//
	// Optional. Default: 5 * time.Minute.
	NonceTTL time.Duration

	// ContextUsernameKey is the key to store the username in the context.
	//
	// Optional. Default: "username".
	ContextUsernameKey string
}

// ConfigDefault is the default config
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

// Helper function to set default values
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
		cfg.Users = ConfigDefault.Users
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
