package resolver

import (
	cli "github.com/smallnest/rpcx/client"
	"net/url"
)

// Builder creates a resolver that will be used to watch name resolution updates.
type Builder interface {
	Build(target *url.URL) (cli.ServiceDiscovery, error)
	Scheme() string
}
