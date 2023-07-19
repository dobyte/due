package direct

import (
	"github.com/dobyte/due/v2/errors"
	cli "github.com/smallnest/rpcx/client"
	"net/url"
)

const scheme = "direct"

type Builder struct {
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Scheme() string {
	return scheme
}

func (b *Builder) Build(target *url.URL) (cli.ServiceDiscovery, error) {
	if target.Scheme != scheme {
		return nil, errors.New("mismatched resolver")
	}

	return cli.NewPeer2PeerDiscovery("tcp@"+target.Host, "")
}
