package resolver

import (
	"net/url"

	cli "github.com/smallnest/rpcx/client"
)

// Builder 解析器构建器接口
// 负责根据目标地址构建服务发现器，并监听名称解析更新
type Builder interface {
	// Build 构建服务发现器
	// @param target *url.URL 目标地址
	// @return @1 cli.ServiceDiscovery 服务发现器
	// @return @2 error 错误信息
	Build(target *url.URL) (cli.ServiceDiscovery, error)
	// Scheme 获取解析器协议
	// @return @1 string 协议名称
	Scheme() string
	// Close 关闭构建器，释放 watch 协程与监听资源
	// @return @1 error 错误信息
	Close() error
}
