package discovery

import (
	cli "github.com/smallnest/rpcx/client"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/registry"
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
