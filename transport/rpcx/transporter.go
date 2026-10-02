package rpcx

import (
	"sync"

	"github.com/dobyte/due/transport/rpcx/v2/internal/client"
	"github.com/dobyte/due/transport/rpcx/v2/internal/logger"
	"github.com/dobyte/due/transport/rpcx/v2/internal/server"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

const name = "rpcx"

// Transporter is a microservice transporter that creates servers and clients and manages the
// lifecycle of client connections.
type Transporter struct {
	opts    *options
	once    sync.Once
	builder *client.Builder
}

var _ transport.Transporter = &Transporter{}

// NewTransporter returns a new transporter configured with the given options.
func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	logger.InitLogger()

	return &Transporter{opts: o}
}

// Name returns the transporter component name.
func (t *Transporter) Name() string {
	return name
}

// SetDefaultDiscovery sets the default service discovery component.
//
// It only takes effect when the client has no discovery component configured yet.
func (t *Transporter) SetDefaultDiscovery(discovery registry.Discovery) {
	if t.opts.client.Discovery == nil {
		t.opts.client.Discovery = discovery
	}
}

// NewServer creates a transport server.
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewClient creates a transport client.
//
// The target may take one of the following forms:
//
//	direct://127.0.0.1:8011                         direct connection by address
//	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070   direct connection by instance ID
//	discovery://service_name                        service discovery by service name
func (t *Transporter) NewClient(target string) (transport.Client, error) {
	t.once.Do(func() {
		t.builder = client.NewBuilder(&t.opts.client)
	})

	cli, err := t.builder.Build(target)
	if err != nil {
		return nil, err
	}

	return client.NewClient(cli), nil
}

// Close closes the transporter and releases all client connections and resources.
func (t *Transporter) Close() error {
	if t.builder == nil {
		return nil
	}

	return t.builder.Close()
}
