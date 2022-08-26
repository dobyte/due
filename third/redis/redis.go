package redis

import "github.com/go-redis/redis/v8"

type Client interface {
	redis.UniversalClient
}

const (
	Nil = redis.Nil
)

type (
	Pong         = redis.Pong
	Message      = redis.Message
	Subscription = redis.Subscription
)

type Option func(o *options)

type options struct {
	addrs      []string // 连接地址
	db         int      // 数据库号
	username   string   // 用户名
	password   string   // 密码
	maxRetries int      // 最大重试次数
}

func NewClient(opts ...Option) Client {
	o := &options{
		addrs: []string{"127.0.0.1:6379"},
	}
	for _, opt := range opts {
		opt(o)
	}

	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:      o.addrs,
		DB:         o.db,
		Username:   o.username,
		Password:   o.password,
		MaxRetries: o.maxRetries,
	})
}

// WithAddrs 设置连接地址
func WithAddrs(addrs ...string) Option { return func(o *options) { o.addrs = addrs } }

// WithDB 设置数据库号
func WithDB(db int) Option { return func(o *options) { o.db = db } }

// WithUsername 设置用户名
func WithUsername(username string) Option { return func(o *options) { o.username = username } }

// WithPassword 设置密码
func WithPassword(password string) Option { return func(o *options) { o.password = password } }

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(maxRetries int) Option { return func(o *options) { o.maxRetries = maxRetries } }
