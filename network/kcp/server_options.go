/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/12 12:57 上午
 * @Desc: TODO
 */

package kcp

import (
	"github.com/symsimmy/due/config"
	"time"
)

const (
	defaultServerAddr                   = ":3554"
	defaultServerMaxMsgLen              = 1024
	defaultServerMaxConnNum             = 5000
	defaultServerHeartbeatCheckInterval = 10
)

const (
	defaultServerAddrKey                   = "config.network.kcp.server.addr"
	defaultServerMaxMsgLenKey              = "config.network.kcp.server.maxMsgLen"
	defaultServerMaxConnNumKey             = "config.network.kcp.server.maxConnNum"
	defaultServerHeartbeatCheckIntervalKey = "config.network.kcp.server.heartbeatCheckInterval"
)

type ServerOption func(o *serverOptions)

type serverOptions struct {
	addr                   string        // 监听地址
	maxMsgLen              int           // 最大消息长度
	maxConnNum             int           // 最大连接数
	heartbeatCheckInterval time.Duration // 心跳检测间隔时间，默认10s
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		addr:                   config.Get(defaultServerAddrKey, defaultServerAddr).String(),
		maxMsgLen:              config.Get(defaultServerMaxMsgLenKey, defaultServerMaxMsgLen).Int(),
		maxConnNum:             config.Get(defaultServerMaxConnNumKey, defaultServerMaxConnNum).Int(),
		heartbeatCheckInterval: config.Get(defaultServerHeartbeatCheckIntervalKey, defaultServerHeartbeatCheckInterval).Duration() * time.Second,
	}
}

// WithServerListenAddr 设置监听地址
func WithServerListenAddr(addr string) ServerOption {
	return func(o *serverOptions) { o.addr = addr }
}

// WithServerMaxConnNum 设置连接的最大连接数
func WithServerMaxConnNum(maxConnNum int) ServerOption {
	return func(o *serverOptions) { o.maxConnNum = maxConnNum }
}

// WithServerMaxMsgLen 设置消息最大长度
func WithServerMaxMsgLen(maxMsgLen int) ServerOption {
	return func(o *serverOptions) { o.maxMsgLen = maxMsgLen }
}

// WithServerHeartbeatInterval 设置心跳检测间隔时间
func WithServerHeartbeatInterval(heartbeatInterval time.Duration) ServerOption {
	return func(o *serverOptions) { o.heartbeatCheckInterval = heartbeatInterval }
}
