/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/13 12:32 上午
 * @Desc: TODO
 */

package etcd

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultAddr        = "127.0.0.1:2379"
	defaultDialTimeout = "5s"
	defaultNamespace   = "services"
	defaultTimeout     = "3s"
	defaultLeaseTTL    = "15s"
)

const (
	defaultAddrsKey       = "etc.registry.etcd.addrs"
	defaultDialTimeoutKey = "etc.registry.etcd.dialTimeout"
	defaultNamespaceKey   = "etc.registry.etcd.namespace"
	defaultTimeoutKey     = "etc.registry.etcd.timeout"
	defaultUsernameKey    = "etc.registry.etcd.username"
	defaultPasswordKey    = "etc.registry.etcd.password"
	defaultLeaseTTLKey    = "etc.registry.etcd.leaseTTL"
)

// Option 服务注册发现配置项
type Option func(o *options)

type options struct {
	// 客户端连接地址
	// 内建客户端配置，默认为[]string{"127.0.0.1:2379"}
	addrs []string

	// 客户端拨号超时时间
	// 内建客户端配置，默认为5秒
	dialTimeout time.Duration

	// 外部客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client *clientv3.Client

	// 命名空间
	// 默认为services
	namespace string

	// 上下文超时时间
	// 默认为3秒
	timeout time.Duration

	// 用户名
	username string

	// 密码
	password string

	// Lease存活时间
	// 默认为15秒
	leaseTTL time.Duration
}

func defaultOptions() *options {
	return &options{
		addrs:       etc.Get(defaultAddrsKey, []string{defaultAddr}).Strings(),
		dialTimeout: etc.Get(defaultDialTimeoutKey, defaultDialTimeout).Duration(),
		namespace:   etc.Get(defaultNamespaceKey, defaultNamespace).String(),
		timeout:     etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		username:    etc.Get(defaultUsernameKey).String(),
		password:    etc.Get(defaultPasswordKey).String(),
		leaseTTL:    etc.Get(defaultLeaseTTLKey, defaultLeaseTTL).Duration(),
	}
}

// WithAddrs 设置客户端连接地址
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithDialTimeout 设置客户端拨号超时时间
// @param dialTimeout time.Duration 客户端拨号超时时间
// @return @1 Option 服务注册发现配置项
func WithDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) { o.dialTimeout = dialTimeout }
}

// WithClient 设置外部客户端
func WithClient(client *clientv3.Client) Option {
	return func(o *options) { o.client = client }
}

// WithNamespace 设置命名空间
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithTimeout 设置上下文超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithUsername 设置用户名
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword 设置密码
// @param password string 密码
// @return @1 Option 服务注册发现配置项
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithLeaseTTL 设置Lease存活时间
// @param leaseTTL time.Duration Lease存活时间
// @return @1 Option 服务注册发现配置项
func WithLeaseTTL(leaseTTL time.Duration) Option {
	return func(o *options) { o.leaseTTL = leaseTTL }
}
