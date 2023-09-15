package direct

import (
	cli "github.com/smallnest/rpcx/client"
	"github.com/symsimmy/due/errors"
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
