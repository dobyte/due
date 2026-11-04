package nacos

import (
	"context"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// Name is the name of the config source.
const Name = "nacos"

// listenRequest is a listen request. It carries a dataId and the channel used to
// report the listen result.
type listenRequest struct {
	dataId string     // Config dataId
	done   chan error // Channel reporting the listen result
}

// Source is a config source backed by Nacos.
type Source struct {
	err      error              // Error encountered while building the client
	opts     *options           // Config options
	ctx      context.Context    // Context
	cancel   context.CancelFunc // Cancel function
	builtin  bool               // Whether the client is built in
	version  uint64             // Current search version
	versions map[string]uint64  // Search version of each dataId
	chListen chan listenRequest // Listen request channel
	chCancel chan string        // Cancel-listen request channel
	watchers sync.Map           // Watcher set
	once     sync.Once          // Ensures the shutdown runs only once
	wg       sync.WaitGroup     // Wait group for goroutine exit
}

// NewSource creates a config source. It builds a Nacos config center client from
// the options and starts the config listen and refresh goroutines. When an
// external client is supplied it takes precedence, and the caller is responsible
// for closing it.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o
	s.ctx, s.cancel = context.WithCancel(o.ctx)
	s.versions = make(map[string]uint64)
	s.chListen = make(chan listenRequest)
	s.chCancel = make(chan string)

	if o.client == nil {
		o.client, s.err = s.buildClient()
		s.builtin = true
	}

	s.wg.Add(2)

	go func() {
		defer s.wg.Done()
		s.listen()
	}()

	go func() {
		defer s.wg.Done()
		s.refresh()
	}()

	return s
}

// Name returns the name of the config source.
func (s *Source) Name() string {
	return Name
}

// Load loads configuration items.
//
// When file is provided it loads only that item; otherwise it pages through and
// loads every item under the group.
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	if len(file) > 0 && file[0] != "" {
		if configuration, err := s.load(ctx, file[0]); err != nil {
			return nil, err
		} else {
			return []*config.Configuration{configuration}, nil
		}
	} else {
		var (
			mu             sync.Mutex
			index          = 1
			configurations = make([]*config.Configuration, 0)
		)

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// continue
			}

			result, err := s.opts.client.SearchConfig(vo.SearchConfigParam{
				Search:   "blur",
				Group:    s.opts.groupName,
				PageNo:   index,
				PageSize: 20,
			})
			if err != nil {
				return nil, err
			}

			switch len(result.PageItems) {
			case 0:
				// ignore
			case 1:
				if configuration, err := s.loadItem(ctx, result.PageItems[0].DataId, result.PageItems[0].Content); err != nil {
					return nil, err
				} else {
					configurations = append(configurations, configuration)
				}
			default:
				wg, _ := task.WithContext(ctx)

				for _, item := range result.PageItems {
					wg.Go(func() error {
						configuration, err := s.loadItem(ctx, item.DataId, item.Content)
						if err != nil {
							return err
						}

						mu.Lock()
						configurations = append(configurations, configuration)
						mu.Unlock()

						return nil
					})
				}

				if err := wg.Wait(); err != nil {
					return nil, err
				}
			}

			if result.PageNumber >= result.PagesAvailable {
				break
			}

			index = result.PageNumber + 1
		}

		return configurations, nil
	}
}

// load loads a single configuration item. It fetches the content from the Nacos
// server by dataId and converts it into the unified configuration structure.
func (s *Source) load(ctx context.Context, file string) (*config.Configuration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	content, err := s.opts.client.GetConfig(vo.ConfigParam{
		DataId: file,
		Group:  s.opts.groupName,
	})
	if err != nil {
		return nil, err
	}

	configuration := s.conv(file, content)

	return configuration, nil
}

// loadItem loads a configuration item. It reuses the content returned by the config search when the
// server provides it and the content does not need decryption; otherwise it fetches the content by
// dataId.
func (s *Source) loadItem(ctx context.Context, file, content string) (*config.Configuration, error) {
	if !s.opts.openKMS && content != "" {
		return s.conv(file, content), nil
	}

	return s.load(ctx, file)
}

// Store stores a configuration item.
//
// It supports only the write-only and read-write modes. After a successful
// publish the server pushes the change to every registered watcher.
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	data := string(content)

	ok, err := s.opts.client.PublishConfig(vo.ConfigParam{
		DataId:  file,
		Group:   s.opts.groupName,
		Content: data,
		Type:    s.parseFileType(file),
	})
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrConfigStoreFailed
	}

	s.onChange(s.opts.namespaceId, s.opts.groupName, file, data)

	return nil
}

// Watch watches configuration items. It creates a new watcher and registers it
// with the config source; the watcher is notified when the configuration changes.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	w := newWatcher(ctx, s)
	s.watchers.Store(w, struct{}{})

	return w, nil
}

// Close closes the config source. It cancels the context and closes the built-in
// client, terminating the listen and refresh goroutines; Close performs the
// shutdown only once.
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	// Ensure the shutdown runs only once to avoid closing the client twice and panicking.
	s.once.Do(func() {
		s.cancel()

		// Wait for the listen and refresh goroutines to exit so that the client is not closed concurrently.
		s.wg.Wait()

		if s.builtin {
			s.opts.client.CloseClient()
		}
	})

	return nil
}

// listen handles listen and cancel-listen requests. It consumes the dataIds
// produced by the search loop and registers or cancels config listening for each.
func (s *Source) listen() {
	if s.err != nil {
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case req, ok := <-s.chListen:
			if !ok {
				return
			}

			err := s.opts.client.ListenConfig(vo.ConfigParam{
				DataId:   req.dataId,
				Group:    s.opts.groupName,
				OnChange: s.onChange,
			})
			if err != nil {
				log.Warnf("%s %s listen failed: %v", s.opts.groupName, req.dataId, err)
			}

			// Report the listen result without blocking so that a requester that has already exited does not stall the loop.
			select {
			case req.done <- err:
			default:
			}
		case dataId, ok := <-s.chCancel:
			if !ok {
				return
			}

			if err := s.opts.client.CancelListenConfig(vo.ConfigParam{
				DataId: dataId,
				Group:  s.opts.groupName,
			}); err != nil {
				log.Warnf("%s %s cancel listen failed: %v", s.opts.groupName, dataId, err)
			}
		}
	}
}

// refresh refreshes the configuration periodically. It runs a config search at a
// fixed interval to detect added or removed configuration items.
func (s *Source) refresh() {
	if s.err != nil {
		return
	}

	s.search()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.search()
		}
	}
}

// search searches for configuration items under the group. It pages through the
// config list, sends a listen request for each added dataId and a cancel request
// for each removed dataId.
func (s *Source) search() {
	s.version++

	index := 1

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// continue
		}

		result, err := s.opts.client.SearchConfig(vo.SearchConfigParam{
			Search:   "blur",
			Group:    s.opts.groupName,
			PageNo:   index,
			PageSize: 20,
		})
		if err != nil {
			log.Warnf("search config list failed: %v", err)
			// Return immediately when the query fails; otherwise the incremented version
			// with an unrefreshed config list would make the cancel loop below treat every
			// item as stale and cancel all listeners in bulk.
			return
		}

		for _, item := range result.PageItems {
			if _, ok := s.versions[item.DataId]; !ok {
				// Skip the version mark when listening fails so that the next search retries it.
				if err := s.requestListen(item.DataId); err != nil {
					continue
				}
			}

			s.versions[item.DataId] = s.version
		}

		if result.PageNumber >= result.PagesAvailable {
			break
		}

		index = result.PageNumber + 1
	}

	for dataId, version := range s.versions {
		if version != s.version {
			select {
			case s.chCancel <- dataId:
			case <-s.ctx.Done():
				return
			}

			// Remove it from the version table right after cancelling so that no duplicate
			// cancel request is sent and the item can be listened to again if it is recreated.
			delete(s.versions, dataId)
		}
	}
}

// requestListen requests config listening. It sends a listen request to the listen
// goroutine and waits for the result.
func (s *Source) requestListen(dataId string) error {
	done := make(chan error, 1)
	req := listenRequest{dataId: dataId, done: done}

	select {
	case s.chListen <- req:
	case <-s.ctx.Done():
		return s.ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

// onChange is the config change callback. It is triggered when the Nacos server
// pushes a configuration change and notifies every registered watcher of the new
// configuration.
func (s *Source) onChange(_, _, file, content string) {
	configuration := s.conv(file, content)

	s.watchers.Range(func(key, value any) bool {
		w := key.(*watcher)
		w.notice(configuration)
		return true
	})
}

// buildClient builds the Nacos config client. It parses the server address list
// and builds a Nacos config center client; an address may carry an optional
// scheme, defaulting to http.
//
// The built-in client lists configurations through the Nacos v3 admin API when
// the server supports it, so a periodic search does not transfer the content of
// every configuration. Servers without the v3 admin API fall back to the SDK
// search at runtime.
func (s *Source) buildClient() (config_client.IConfigClient, error) {
	serverConfigs, endpoint, err := newServerConfigs(s.opts.urls)

	if len(serverConfigs) == 0 {
		if err != nil {
			return nil, err
		} else {
			return nil, errors.New("invalid server urls")
		}
	} else {
		if err != nil {
			log.Warnf("%s parse failed: %v", endpoint, err)
		}

		clientConfig := newClientConfig(s.opts)
		client, err := clients.NewConfigClient(vo.NacosClientParam{
			ServerConfigs: serverConfigs,
			ClientConfig:  &clientConfig,
		})
		if err != nil {
			return nil, err
		}

		return newListConfigClient(client, s.opts, s.ctx), nil
	}
}

// newServerConfigs parses the server address list. An address may carry an
// optional scheme, defaulting to http.
func newServerConfigs(urls []string) ([]constant.ServerConfig, string, error) {
	configs := make([]constant.ServerConfig, 0, len(urls))

	var (
		err      error
		endpoint string
	)

	for _, v := range urls {
		if !strings.Contains(v, "://") {
			v = "http://" + v
		}

		raw, e := url.Parse(v)
		if e != nil {
			err, endpoint = e, v
			continue
		}

		host, p, e := net.SplitHostPort(raw.Host)
		if e != nil {
			err, endpoint = e, v
			continue
		}

		port, e := strconv.ParseUint(p, 10, 64)
		if e != nil {
			err, endpoint = e, v
			continue
		}

		configs = append(configs, constant.ServerConfig{
			Scheme:      raw.Scheme,
			ContextPath: raw.Path,
			IpAddr:      host,
			Port:        port,
		})
	}

	return configs, endpoint, err
}

// newClientConfig builds the Nacos client config from the source options.
func newClientConfig(o *options) constant.ClientConfig {
	return constant.ClientConfig{
		TimeoutMs:            uint64(o.timeout.Milliseconds()),
		NamespaceId:          o.namespaceId,
		ClusterName:          o.clusterName,
		Endpoint:             o.endpoint,
		RegionId:             o.regionId,
		AccessKey:            o.accessKey,
		SecretKey:            o.secretKey,
		OpenKMS:              o.openKMS,
		CacheDir:             o.cacheDir,
		Username:             o.username,
		Password:             o.password,
		LogDir:               o.logDir,
		LogLevel:             o.logLevel,
		NotLoadCacheAtStart:  true,
		UpdateCacheWhenEmpty: true,
	}
}

// parseFileType converts the configuration type. It maps the file extension of
// the dataId to a Nacos config type supporting only json, xml and yaml; any other
// format falls back to the default text type.
func (s *Source) parseFileType(file string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(file), ".")) {
	case "json":
		return "json"
	case "xml":
		return "xml"
	case "yml", "yaml":
		return "yaml"
	default:
		return "text"
	}
}

// conv converts a configuration. It converts a dataId and its content into the
// unified configuration structure, using the file extension of the dataId as the
// config format.
func (s *Source) conv(file, content string) *config.Configuration {
	ext := filepath.Ext(file)

	return &config.Configuration{
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  []byte(content),
		FullPath: file,
	}
}
