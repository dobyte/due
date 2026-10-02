package kafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultAddr              = "127.0.0.1:9092"
	defaultPrefix            = "due:eventbus"
	defaultAutoCreateTopic   = true
	defaultPartitions        = 1
	defaultReplicationFactor = 1
	defaultStaleDuration     = 0
)

const (
	defaultAddrsKey             = "etc.eventbus.kafka.addrs"
	defaultPrefixKey            = "etc.eventbus.kafka.prefix"
	defaultVersionKey           = "etc.eventbus.kafka.version"
	defaultAutoCreateTopicKey   = "etc.eventbus.kafka.autoCreateTopic"
	defaultPartitionsKey        = "etc.eventbus.kafka.partitions"
	defaultReplicationFactorKey = "etc.eventbus.kafka.replicationFactor"
	defaultStaleDurationKey     = "etc.eventbus.kafka.staleDuration"
)

// Option configures the eventbus.
type Option func(o *options)

// options holds the eventbus options.
type options struct {
	ctx context.Context

	// Client connection addresses.
	// Built-in client configuration, defaults to []string{"127.0.0.1:9092"}.
	addrs []string

	// Kafka version, defaults to no version.
	version string

	// Prefix.
	// Key prefix, defaults to due:eventbus.
	prefix string

	// Client.
	// External client configuration; when set, the external client is preferred. Defaults to nil.
	client sarama.Client

	// Automatically create topic.
	// When true, the topic is created automatically if it does not exist. Defaults to true.
	autoCreateTopic bool

	// Number of partitions.
	// Number of partitions used when creating a topic automatically. Defaults to 1.
	partitions int32

	// Replication factor.
	// Replication factor used when creating a topic automatically. Defaults to 1.
	replicationFactor int16

	// Stale duration.
	// Messages older than this are dropped. Defaults to 0, which keeps stale messages.
	staleDuration time.Duration
}

// defaultOptions returns the default options.
func defaultOptions() *options {
	return &options{
		ctx:               context.Background(),
		addrs:             etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		prefix:            etc.Get(defaultPrefixKey, defaultPrefix).String(),
		version:           etc.Get(defaultVersionKey).String(),
		autoCreateTopic:   etc.Get(defaultAutoCreateTopicKey, defaultAutoCreateTopic).Bool(),
		partitions:        etc.Get(defaultPartitionsKey, defaultPartitions).Int32(),
		replicationFactor: etc.Get(defaultReplicationFactorKey, defaultReplicationFactor).Int16(),
		staleDuration:     etc.Get(defaultStaleDurationKey, defaultStaleDuration).Duration(),
	}
}

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithAddrs sets the connection addresses.
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithPrefix sets the prefix.
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithVersion sets the Kafka version.
func WithVersion(version string) Option {
	return func(o *options) { o.version = version }
}

// WithClient sets the external client.
func WithClient(client sarama.Client) Option {
	return func(o *options) { o.client = client }
}

// WithAutoCreateTopic sets whether to create topics automatically.
func WithAutoCreateTopic(autoCreateTopic bool) Option {
	return func(o *options) { o.autoCreateTopic = autoCreateTopic }
}

// WithStaleDuration sets the stale duration of messages.
func WithStaleDuration(staleDuration time.Duration) Option {
	return func(o *options) { o.staleDuration = staleDuration }
}

// WithPartitions sets the number of partitions used when creating a topic automatically.
func WithPartitions(partitions int32) Option {
	return func(o *options) { o.partitions = partitions }
}

// WithReplicationFactor sets the replication factor used when creating a topic automatically.
func WithReplicationFactor(replicationFactor int16) Option {
	return func(o *options) { o.replicationFactor = replicationFactor }
}
