package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Option func(*options)

type options struct {
	addrs      []string
	db         int
	username   string
	password   string
	maxRetries int
	prefix     string
	client     redis.UniversalClient
}

func defaultOptions() *options {
	return &options{
		addrs:      []string{"127.0.0.1:6379"},
		maxRetries: 3,
		prefix:     "due:job:{all}",
	}
}

func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

func WithDB(db int) Option {
	return func(o *options) { o.db = db }
}

func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

func WithMaxRetries(maxRetries int) Option {
	return func(o *options) { o.maxRetries = maxRetries }
}

func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

func WithClient(client redis.UniversalClient) Option {
	return func(o *options) { o.client = client }
}

func clientFromOptions(ctx context.Context, o *options) (redis.UniversalClient, bool) {
	if o.client != nil {
		return o.client, false
	}

	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:      o.addrs,
		DB:         o.db,
		Username:   o.username,
		Password:   o.password,
		MaxRetries: o.maxRetries,
	}), true
}
