package resolver

import (
	cli "github.com/smallnest/rpcx/client"
	"net/url"
)

type Target = *url.URL

// Builder creates a resolver that will be used to watch name resolution updates.
type Builder interface {
	Build(target Target) (cli.ServiceDiscovery, error)
	Scheme() string
}
