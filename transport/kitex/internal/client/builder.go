package client

import (
	"github.com/cloudwego/kitex/client"
	"github.com/dobyte/due/v2/registry"
	"net/url"
)

type Builder struct {
}

type Options struct {
	Discovery registry.Discovery
}

func NewBuilder(opts *Options) *Builder {

}

// Build 构建客户端
func (b *Builder) Build(target string) (client.Client, error) {
	if b.err != nil {
		return nil, b.err
	}

	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" {
		u.Scheme = defaultBuilder
		target = u.String()
	}

	client.NewClient()
}
