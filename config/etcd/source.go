package etcd

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/config"
	coreconfig "github.com/dobyte/due/v2/core/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Name is the config source name.
const Name = "etcd"

// Source is the etcd config source.
type Source struct {
	err      error        // Error returned when the client was built
	opts     *options     // Configuration options
	builtin  bool         // Whether the client is built in
	opsMu    sync.RWMutex // Guards the client against the in-flight Load, Store and Watch calls
	watchers sync.Map     // Watcher set
	closed   atomic.Bool  // Whether the source has been closed
}

// NewSource creates a config source. It builds an etcd config center client from
// the given options, creating a built-in client when no external client is
// specified.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	o.files = coreconfig.NormalizeFiles(o.files)

	for _, file := range coreconfig.UnmatchableFiles(o.files) {
		log.Warnf("config file entry %q is absolute or contains a directory traversal, it will never be matched", file)
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

// Load loads configuration items. When the first entry of file is not empty, only that
// configuration item is loaded; otherwise every configuration item under the base path is loaded.
// Further entries are ignored.
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	s.opsMu.RLock()
	defer s.opsMu.RUnlock()

	if s.closed.Load() {
		return nil, errors.ErrClosedConfigSource
	}

	var (
		key  = s.opts.path
		opts []clientv3.OpOption
	)

	if len(file) > 0 && file[0] != "" {
		name := strings.TrimPrefix(file[0], "/")

		if err := coreconfig.ValidateFileEntry(name); err != nil {
			return nil, err
		}

		if !s.matchFile(name) {
			return nil, errors.ErrFileNotConfigured
		}

		key += name
	} else {
		opts = append(opts, clientv3.WithPrefix())
	}

	res, err := s.opts.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	configs := make([]*config.Configuration, 0, len(res.Kvs))
	for _, kv := range res.Kvs {
		c := s.parseKV(kv.Key, kv.Value)
		if !s.matchFile(c.Path) {
			continue
		}

		configs = append(configs, c)
	}

	return configs, nil
}

// Store stores a configuration item. Only the write-only and read-write modes are
// supported; the other modes report an operation permission error.
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	s.opsMu.RLock()
	defer s.opsMu.RUnlock()

	if s.closed.Load() {
		return errors.ErrClosedConfigSource
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	name := strings.TrimPrefix(file, "/")

	if err := coreconfig.ValidateFileEntry(name); err != nil {
		return err
	}

	if !s.matchFile(name) {
		return errors.ErrFileNotConfigured
	}

	key := s.opts.path + name
	_, err := s.opts.client.Put(ctx, key, string(content))
	return err
}

// parseKV parses an etcd key-value pair into the unified configuration structure.
func (s *Source) parseKV(key, value []byte) *config.Configuration {
	fullPath := string(key)
	relPath := strings.TrimPrefix(fullPath, s.opts.path)
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

// matchFile reports whether file is selected by the configured file list. When no
// file is configured, every file is selected.
func (s *Source) matchFile(file string) bool {
	if len(s.opts.files) == 0 {
		return true
	}

	return coreconfig.MatchFile(s.opts.files, file)
}

// Watch watches configuration items. It pulls the full configuration once as the
// initial snapshot and then creates a watcher to observe subsequent changes. It
// reports [errors.ErrClosedConfigSource] once the source has been closed.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	s.opsMu.RLock()
	defer s.opsMu.RUnlock()

	if s.closed.Load() {
		return nil, errors.ErrClosedConfigSource
	}

	// Pull the full configuration once to use as the initial watch snapshot and to
	// record the starting revision of the watch.
	tctx, cancel := context.WithTimeout(ctx, s.opts.timeout)
	res, err := s.opts.client.Get(tctx, s.opts.path, clientv3.WithPrefix())
	cancel()
	if err != nil {
		log.Warnf("etcd watch get failed: %v", err)
		res = nil
	}

	w := newWatcher(ctx, s, res)
	s.watchers.Store(w, struct{}{})

	// The source may have been closed between the check above and the registration, in which case
	// the shutdown no longer visits the watcher. Stop it so that it does not stay registered.
	if s.closed.Load() {
		_ = w.Stop()

		return nil, errors.ErrClosedConfigSource
	}

	return w, nil
}

// Close closes the resources. It stops the registered watchers and closes the
// client connection for a built-in client; an external client is closed by the
// caller.
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	// Wait for the in-flight Load, Store and Watch calls before the watchers and the client are
	// closed, so that the client is never destroyed under a running request.
	s.opsMu.Lock()
	defer s.opsMu.Unlock()

	s.watchers.Range(func(key, _ any) bool {
		_ = key.(*watcher).Stop()

		return true
	})

	if s.builtin {
		return s.opts.client.Close()
	}

	return nil
}
