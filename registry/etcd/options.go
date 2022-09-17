/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/13 12:32 上午
 * @Desc: TODO
 */

package etcd

import (
	"context"
	"time"
)

type Option func(o *options)

type options struct {
	ctx           context.Context // 上下文，默认context.Background
	addrs         []string        // 服务器连接地址，默认为[]string{"localhost:2379"}
	dialTimeout   time.Duration   // 拨号超时时间，默认为5秒
	namespace     string          // 命名空间，默认为services
	timeout       time.Duration   // 上下文超时时间，默认为3秒
	retryTimes    int             // 心跳重试次数，默认为3次
	retryInterval time.Duration   // 心跳重试间隔，默认为10秒
}

// WithAddrs 设置服务器连接地址
func WithAddrs(addrs ...string) Option {
	return func(o *options) { o.addrs = addrs }
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithTimeout 设置拨号超时时间
func WithDialTimeout(dialTimeout time.Duration) Option {
	return func(o *options) { o.dialTimeout = dialTimeout }
}

// WithNamespace 设置命名空间
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithTimeout 设置上下文超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithRetryTimes 设置心跳重试次数
func WithRetryTimes(retryTimes int) Option {
	return func(o *options) { o.retryTimes = retryTimes }
}

// WithRetryInterval 设置心跳重试间隔时间
func WithRetryInterval(retryInterval time.Duration) Option {
	return func(o *options) { o.retryInterval = retryInterval }
}
