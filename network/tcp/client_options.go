package tcp

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultClientAddr              = "127.0.0.1:3553"
	defaultClientTimeout           = "5s"
	defaultClientHeartbeatInterval = "10s"
)

const (
	defaultClientAddrKey              = "etc.network.tcp.client.addr"
	defaultClientCAFileKey            = "etc.network.tcp.client.caFile"
	defaultClientServerNameKey        = "etc.network.tcp.client.serverName"
	defaultClientTimeoutKey           = "etc.network.tcp.client.timeout"
	defaultClientHeartbeatIntervalKey = "etc.network.tcp.client.heartbeatInterval"
)

type ClientOption func(o *clientOptions)

type clientOptions struct {
	addr              string        // 地址
	caFile            string        // CA证书文件
	serverName        string        // 服务器名称
	timeout           time.Duration // 拨号超时时间，默认5s
	heartbeatInterval time.Duration // 心跳间隔时间，默认10s
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		addr:              etc.Get(defaultClientAddrKey, defaultClientAddr).String(),
		timeout:           etc.Get(defaultClientTimeoutKey, defaultClientTimeout).Duration(),
		caFile:            etc.Get(defaultClientCAFileKey).String(),
		serverName:        etc.Get(defaultClientServerNameKey).String(),
		heartbeatInterval: etc.Get(defaultClientHeartbeatIntervalKey, defaultClientHeartbeatInterval).Duration(),
	}
}

// WithClientAddr 设置拨号地址
func WithClientAddr(addr string) ClientOption {
	return func(o *clientOptions) { o.addr = addr }
}

// WithClientTimeout 设置拨号超时时间
func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) { o.timeout = timeout }
}

// WithClientCredentials 设置CA证书和校验域名
func WithClientCredentials(caFile string, serverName string) ClientOption {
	return func(o *clientOptions) { o.caFile, o.serverName = caFile, serverName }
}

// WithClientHeartbeatInterval 设置心跳间隔时间
func WithClientHeartbeatInterval(heartbeatInterval time.Duration) ClientOption {
	return func(o *clientOptions) { o.heartbeatInterval = heartbeatInterval }
}
