package consul

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/hashicorp/consul/api"
)

// Name is the config source name.
const Name = "consul"

// Source is the consul config source.
type Source struct {
	err       error           // Error returned when the client was built
	opts      *options        // Configuration options
	builtin   bool            // Whether the client is built in
	transport *http.Transport // Underlying transport of the built-in client
}

// NewSource creates a config source. It builds a Consul config center client from
// the given options, creating a built-in client when no external client is
// specified.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o

	// Normalize the path by trimming the leading and trailing slashes. Warn and
	// fall back to the default value when the path is empty, so that an empty path
	// does not make List or the watch cover every Consul key.
	path := strings.Trim(s.opts.path, "/")
	if path == "" {
		log.Warnf("invalid config path, use default path: %s", defaultPath)
		path = strings.Trim(defaultPath, "/")
	}
	s.opts.path = path

	if o.client == nil {
		c := api.DefaultConfig()
		if o.addr != "" {
			c.Address = o.addr
		}

		s.builtin = true
		s.transport = c.Transport
		s.opts.client, s.err = api.NewClient(c)
	}

	return s
}

// Name returns the config source name.
func (s *Source) Name() string {
	return Name
}

// Load loads configuration items. When file is provided, only the specified
// configuration item is loaded; otherwise every configuration item under the base
// path is loaded.
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	// When file is provided, query by the exact key and return only the target
	// configuration item.
	if len(file) > 0 && file[0] != "" {
		key := s.opts.path + "/" + strings.TrimPrefix(file[0], "/")

		kv, _, err := s.opts.client.KV().Get(key, (&api.QueryOptions{}).WithContext(ctx))
		if err != nil {
			return nil, err
		}

		if kv == nil {
			return nil, nil
		}

		return []*config.Configuration{s.parseKV(kv.Key, kv.Value)}, nil
	}

	// When file is not provided, load every configuration item under the base path.
	kvs, _, err := s.opts.client.KV().List(s.opts.path+"/", (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, err
	}

	configs := make([]*config.Configuration, 0, len(kvs))
	for _, kv := range kvs {
		configs = append(configs, s.parseKV(kv.Key, kv.Value))
	}

	return configs, nil
}

// Store stores a configuration item. Only the write-only and read-write modes are
// supported; the other modes report an operation permission error.
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	key := s.opts.path + "/" + strings.TrimPrefix(file, "/")

	_, err := s.opts.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: content,
	}, (&api.WriteOptions{}).WithContext(ctx))

	return err
}

// parseKV parses a Consul key-value pair into the unified configuration structure.
func (s *Source) parseKV(key string, value []byte) *config.Configuration {
	fullPath := key
	relPath := strings.TrimPrefix(fullPath, s.opts.path)
	relPath = strings.TrimPrefix(relPath, "/")
	file := path.Base(fullPath)
	ext := path.Ext(file)

	return &config.Configuration{
		Path:     relPath,
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  value,
		FullPath: fullPath,
	}
}

// Watch watches configuration items. It creates a watcher that observes
// configuration changes under the base path.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	return newWatcher(ctx, s)
}

// Close closes the config source. It closes the idle connections of the built-in
// client's transport; an external client is closed by the caller.
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	if s.builtin && s.transport != nil {
		s.transport.CloseIdleConnections()
	}

	return nil
}
