package discovery

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
	"net/url"
)

const scheme = "discovery"

type Builder struct {
	dis registry.Discovery
}

func NewBuilder(dis registry.Discovery) *Builder {
	return &Builder{dis: dis}
}

func (b *Builder) Scheme() string {
	return scheme
}

func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
	if target.Scheme != scheme {
		return nil, errors.New("mismatched resolver")
	}

	return newResolver(b.dis, target.Host)
}
