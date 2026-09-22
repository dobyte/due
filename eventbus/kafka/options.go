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

// Option 事件总线选项
type Option func(o *options)

// options 事件总线配置
type options struct {
	ctx context.Context

	// 客户端连接地址
	// 内建客户端配置，默认为[]string{"127.0.0.1:9092"}
	addrs []string

	// Kafka版本，默认为无版本
	version string

	// 前缀
	// key前缀，默认为due:eventbus
	prefix string

	// 客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client sarama.Client

	// 自动创建topic
	// 当为true时，若不存在该主题，会自动创建，默认为true
	autoCreateTopic bool

	// 分区数量
	// 自动创建topic时使用的分区数量，默认为1
	partitions int32

	// 复制因子
	// 自动创建topic时使用的复制因子，默认为1
	replicationFactor int16

	// 过期时间
	// 超过此时间的消息将被丢弃，默认为0表示不丢弃过期消息
	staleDuration time.Duration
}

// defaultOptions 获取默认配置
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

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithAddrs 设置连接地址
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithPrefix 设置前缀
func WithPrefix(prefix string) Option {
	return func(o *options) { o.prefix = prefix }
}

// WithVersion 设置Kafka版本
func WithVersion(version string) Option {
	return func(o *options) { o.version = version }
}

// WithClient 设置外部客户端
func WithClient(client sarama.Client) Option {
	return func(o *options) { o.client = client }
}

// WithAutoCreateTopic 设置自动创建topic
func WithAutoCreateTopic(autoCreateTopic bool) Option {
	return func(o *options) { o.autoCreateTopic = autoCreateTopic }
}

// WithStaleDuration 设置消息过期时间
func WithStaleDuration(staleDuration time.Duration) Option {
	return func(o *options) { o.staleDuration = staleDuration }
}

// WithPartitions 设置自动创建topic时的分区数量
func WithPartitions(partitions int32) Option {
	return func(o *options) { o.partitions = partitions }
}

// WithReplicationFactor 设置自动创建topic时的复制因子
func WithReplicationFactor(replicationFactor int16) Option {
	return func(o *options) { o.replicationFactor = replicationFactor }
}
