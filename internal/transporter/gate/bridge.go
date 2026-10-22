package gate

import (
	"sync"

	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/bridge"
)

// Resolve selects local delivery only for a matching discovered incarnation.
func (b *Builder) Resolve(target string) (Peer, error) {
	b.mu.RLock()
	closed := b.closed
	cached := b.locals[target]
	b.mu.RUnlock()
	if closed {
		return nil, errors.ErrClientClosed
	}
	if cached != nil && !b.opts.DisableLocal {
		if !cached.endpoint.Closed() {
			return cached, nil
		}
		b.mu.Lock()
		delete(b.locals, target)
		b.mu.Unlock()
		return nil, errors.ErrServerClosed
	}
	d := b.opts.Directory
	if d == nil {
		d = bridge.Default()
	}
	ep, addr, err := d.Resolve(target, "gate", b.opts.DisableLocal)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return b.Build(addr)
	}
	p, ok := ep.Provider().(Provider)
	if !ok {
		return nil, errors.ErrInvalidArgument
	}
	client := &localClient{endpoint: ep, provider: p, opts: *b.opts}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, errors.ErrClientClosed
	}
	// Bound references retained across repeated instance replacement.
	if len(b.locals) >= 256 {
		clear(b.locals)
	}
	b.locals[target] = client
	b.mu.Unlock()
	return client, nil
}

type localServer struct {
	mu       sync.Mutex
	endpoint *bridge.Endpoint
	stopped  bool
	stopOnce sync.Once
	stopErr  error
}

// Start opens the remote listener unless disabled and registers the local provider.
func (s *Server) Start() error {
	s.local.mu.Lock()
	defer s.local.mu.Unlock()
	if s.local.stopped {
		return errors.ErrServerClosed
	}
	if s.local.endpoint != nil {
		return nil
	}
	if !s.options.DisableRemote {
		if err := s.Server.Start(); err != nil {
			return err
		}
	}
	if !s.options.DisableLocal {
		d := s.options.Directory
		if d == nil {
			d = bridge.Default()
		}
		capacity := int(s.options.WriteQueueSize)
		if capacity < 1 {
			capacity = 4096
		}
		s.local.endpoint = d.Register("gate", s.provider, 4, capacity)
	}
	return nil
}

// Stop joins local workers before the caller destroys provider resources.
func (s *Server) Stop() error {
	s.local.stopOnce.Do(func() {
		s.local.mu.Lock()
		s.local.stopped = true
		ep := s.local.endpoint
		s.local.mu.Unlock()
		if ep != nil {
			ep.Close()
		}
		if !s.options.DisableRemote {
			s.local.stopErr = s.Server.Stop()
		}
	})
	return s.local.stopErr
}

// Metadata returns an independent map containing the incarnation token.
func (s *Server) Metadata(metadata map[string]string) map[string]string {
	s.local.mu.Lock()
	defer s.local.mu.Unlock()
	if s.local.endpoint == nil {
		return metadata
	}
	return s.local.endpoint.Metadata(metadata)
}

// Endpoint returns a local-only endpoint when the listener is disabled.
func (s *Server) Endpoint() *endpoint.Endpoint {
	if s.options.DisableRemote {
		return endpoint.NewEndpoint("local", "gate", false)
	}
	return s.Server.Endpoint()
}

// ListenAddr describes the active transport; local-only mode has no TCP listener.
func (s *Server) ListenAddr() string {
	if s.options.DisableRemote {
		return s.Endpoint().String()
	}
	return s.Server.ListenAddr()
}

// ExposeAddr describes the endpoint advertised to discovery.
func (s *Server) ExposeAddr() string {
	if s.options.DisableRemote {
		return s.Endpoint().String()
	}
	return s.Server.ExposeAddr()
}

// Scheme identifies the selected server transport.
func (s *Server) Scheme() string {
	if s.options.DisableRemote {
		return "local"
	}
	return s.Server.Scheme()
}
