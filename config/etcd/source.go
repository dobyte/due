package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Name is the config source name.
const Name = "etcd"

// Source is the etcd config source.
type Source struct {
	err     error    // Error returned when the client was built
	opts    *options // Configuration options
	builtin bool     // Whether the client is built in
}

// NewSource creates a config source. It builds an etcd config center client from
// the given options, creating a built-in client when no external client is
// specified.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o

	// Normalize the path and force a trailing slash so that WithPrefix does not
	// match keys of sibling namespaces (such as /config2 or /confighost).
	path := strings.Trim(o.path, "/")
	if path == "" {
		log.Warnf("invalid config path, use default path: %s", defaultPath)
		path = strings.Trim(defaultPath, "/")
	}
	s.opts.path = fmt.Sprintf("/%s/", path)

	if o.client == nil {
		s.builtin = true
		o.client, s.err = clientv3.New(clientv3.Config{
			Endpoints:   o.addrs,
			DialTimeout: o.dialTimeout,
			Username:    o.username,
			Password:    o.password,
		})
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

	var (
		key  = s.opts.path
		opts []clientv3.OpOption
	)

	if len(file) > 0 && file[0] != "" {
		key += strings.TrimPrefix(file[0], "/")
	} else {
		opts = append(opts, clientv3.WithPrefix())
	}

	res, err := s.opts.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	configs := make([]*config.Configuration, 0, len(res.Kvs))
	for _, kv := range res.Kvs {
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

	key := s.opts.path + strings.TrimPrefix(file, "/")
	_, err := s.opts.client.Put(ctx, key, string(content))
	return err
}

// parseKV parses an etcd key-value pair into the unified configuration structure.
func (s *Source) parseKV(key, value []byte) *config.Configuration {
	fullPath := string(key)
	path := strings.TrimPrefix(fullPath, s.opts.path)
	file := filepath.Base(fullPath)
	ext := filepath.Ext(file)

	return &config.Configuration{
		Path:     path,
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  value,
		FullPath: fullPath,
	}
}

// Watch watches configuration items. It pulls the full configuration once as the
// initial snapshot and then creates a watcher to observe subsequent changes.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	// Pull the full configuration once to use as the initial watch snapshot and to
	// record the starting revision of the watch.
	res, err := s.opts.client.Get(ctx, s.opts.path, clientv3.WithPrefix())
	if err != nil {
		log.Warnf("etcd watch get failed: %v", err)
		res = nil
	}

	return newWatcher(ctx, s, res), nil
}

// Close closes the resources. It closes the client connection for a built-in
// client; an external client is closed by the caller.
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	if s.builtin {
		return s.opts.client.Close()
	}

	return nil
}
