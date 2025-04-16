package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver/direct"
	"github.com/dobyte/due/transport/rpcx/v2/internal/resolver/discovery"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	cli "github.com/smallnest/rpcx/client"
	proto "github.com/smallnest/rpcx/protocol"
	"golang.org/x/sync/singleflight"
	"net/url"
	"os"
	"sync"
)

const defaultPoolSize = 10

type Builder struct {
	err      error
	opts     *Options
	dialOpts cli.Option
	builders map[string]resolver.Builder
	sfg      singleflight.Group
	pools    sync.Map
}

type Options struct {
	PoolSize   int
	CertFile   string
	ServerName string
	Discovery  registry.Discovery
	FailMode   cli.FailMode
}

func NewBuilder(opts *Options) *Builder {
	b := &Builder{}
	b.opts = opts
	b.builders = make(map[string]resolver.Builder)
	b.dialOpts = cli.DefaultOption
	b.dialOpts.CompressType = proto.Gzip
	b.RegisterBuilder(direct.NewBuilder(opts.Discovery))
	if opts.Discovery != nil {
		b.RegisterBuilder(discovery.NewBuilder(opts.Discovery))
	}

	if opts.CertFile != "" && opts.ServerName != "" {
		b.dialOpts.TLSConfig, b.err = newClientTLSFromFile(opts.CertFile, opts.ServerName)
	}

	return b
}

func newClientTLSFromFile(certFile string, serverName string) (*tls.Config, error) {
	b, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(b) {
		return nil, fmt.Errorf("credentials: failed to append certificates")
	}

	return &tls.Config{ServerName: serverName, RootCAs: cp}, nil
}

// RegisterBuilder 注册构建器
func (b *Builder) RegisterBuilder(builder resolver.Builder) {
	b.builders[builder.Scheme()] = builder
}

// Build 建立Discovery
func (b *Builder) Build(target string) (*cli.OneClient, error) {
	if b.err != nil {
		return nil, b.err
	}

	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	val, ok := b.pools.Load(target)
	if ok {
		return val.(*cli.OneClientPool).Get(), nil
	}

	val, err, _ = b.sfg.Do(target, func() (interface{}, error) {
		builder, ok := b.builders[u.Scheme]
		if !ok {
			return nil, errors.ErrMissingResolver
		}

		dis, err := builder.Build(u)
		if err != nil {
			return nil, err
		}

		size := b.opts.PoolSize
		if size <= 0 {
			size = defaultPoolSize
		}

		pool := cli.NewOneClientPool(size, cli.Failtry, cli.RoundRobin, dis, b.dialOpts)

		b.pools.Store(target, pool)

		return pool, nil
	})
	if err != nil {
		return nil, err
	}

	return val.(*cli.OneClientPool).Get(), nil
}
