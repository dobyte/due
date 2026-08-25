package resolver

import (
	"net/url"

	cli "github.com/smallnest/rpcx/client"
)

// Builder creates a resolver that will be used to watch name resolution updates.
type Builder interface {
	Build(target *url.URL) (cli.ServiceDiscovery, error)
	Scheme() string
	// Close 关闭构建器，释放 watch 协程与监听资源
	Close() error
}
