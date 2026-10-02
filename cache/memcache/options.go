package memcache

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultAddr          = "127.0.0.1:11211"
	defaultPrefix        = "due:cache"
	defaultNilValue      = "cache@nil"
	defaultNilExpiration = "10s"
	defaultMinExpiration = "1h"
	defaultMaxExpiration = "24h"
)

const (
	defaultAddrsKey         = "etc.cache.memcache.addrs"
	defaultPrefixKey        = "etc.cache.memcache.prefix"
	defaultNilValueKey      = "etc.cache.memcache.nilValue"
	defaultNilExpirationKey = "etc.cache.memcache.nilExpiration"
	defaultMinExpirationKey = "etc.cache.memcache.minExpiration"
	defaultMaxExpirationKey = "etc.cache.memcache.maxExpiration"
)

type Option func(o *options)

type options struct {
	// Client connection addresses.
	// Built-in client configuration; defaults to []string{"127.0.0.1:11211"}.
	addrs []string

	// Client.
	// External client configuration. When an external client is set, it takes precedence over the
	// built-in one; defaults to nil.
	client *memcache.Client

	// Prefix.
	// Key prefix; defaults to cache.
	prefix string

	// Nil value; defaults to cache@nil.
	nilValue string

	// Nil value expiration time; defaults to 10s.
	nilExpiration time.Duration

	// Minimum expiration time; defaults to 1h.
	minExpiration time.Duration

	// Maximum expiration time; defaults to 24h.
	maxExpiration time.Duration
}

func defaultOptions() *options {
	return &options{
		addrs:         etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		prefix:        etc.Get(defaultPrefixKey, defaultPrefix).String(),
		nilValue:      etc.Get(defaultNilValueKey, defaultNilValue).String(),
		nilExpiration: etc.Get(defaultNilExpirationKey, defaultNilExpiration).Duration(),
		minExpiration: etc.Get(defaultMinExpirationKey, defaultMinExpiration).Duration(),
		maxExpiration: etc.Get(defaultMaxExpirationKey, defaultMaxExpiration).Duration(),
	}
}

// WithAddrs sets the connection addresses. It accepts one or more Memcache node addresses.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithClient sets an external client. The external client takes precedence over the built-in one.
func WithClient(client *memcache.Client) Option {
	return func(o *options) { o.client = client }
}

// WithPrefix sets the key prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithNilValue sets the placeholder value that marks a cached nil.
func WithNilValue(nilValue string) Option {
	return func(o *options) { o.nilValue = nilValue }
}

// WithNilExpiration sets the expiration time of a cached nil. Non-positive values are ignored.
func WithNilExpiration(nilExpiration time.Duration) Option {
	return func(o *options) {
		if nilExpiration > 0 {
			o.nilExpiration = nilExpiration
		}
	}
}

// WithMinExpiration sets the minimum expiration time. Non-positive values are ignored.
func WithMinExpiration(minExpiration time.Duration) Option {
	return func(o *options) {
		if minExpiration > 0 {
			o.minExpiration = minExpiration
		}
	}
}

// WithMaxExpiration sets the maximum expiration time. Non-positive values are ignored.
func WithMaxExpiration(maxExpiration time.Duration) Option {
	return func(o *options) {
		if maxExpiration > 0 {
			o.maxExpiration = maxExpiration
		}
	}
}
