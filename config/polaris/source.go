package polaris

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
)

// Name is the name of the config source.
const Name = "polaris"

// Source is a config source backed by Polaris.
type Source struct {
	err            error              // Error encountered while building the client
	opts           *options           // Config options
	ctx            context.Context    // Context
	cancel         context.CancelFunc // Cancel function
	builtin        bool               // Whether the client is built in
	version        uint64             // Current search version
	versions       map[string]uint64  // Search version of each config file
	chListen       chan string        // Listen request channel
	chCancel       chan string        // Cancel-listen request channel
	watchers       sync.Map           // Watcher set
	once           sync.Once          // Ensures the shutdown runs only once
	wg             sync.WaitGroup     // Wait group for goroutine exit
	searchDisabled bool               // Whether group search is unavailable
	configClient   api.ConfigFileAPI  // Config file client
	groupClient    api.ConfigGroupAPI // Config group client
	subscribed     sync.Map           // Config files that have registered a change listener
}

// NewSource creates a config source. It builds a Polaris config center client from
// the options and starts the config listen and refresh goroutines. When an
// external SDK context is supplied it takes precedence, and the caller is
// responsible for destroying it.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o
	s.ctx, s.cancel = context.WithCancel(o.ctx)
	s.versions = make(map[string]uint64)
	s.chListen = make(chan string)
	s.chCancel = make(chan string)

	if o.client == nil {
		o.client, s.err = s.buildClient()
		s.builtin = true
	}

	if s.err == nil {
		s.configClient = api.NewConfigFileAPIBySDKContext(o.client)
		s.groupClient = api.NewConfigGroupAPIBySDKContext(o.client)
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
// When file is provided it loads only that item; otherwise it loads every
// published item under the group.
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
			configurations = make([]*config.Configuration, 0)
		)

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		group, err := s.groupClient.GetConfigGroup(s.opts.namespace, s.opts.group)
		if err != nil {
			return nil, err
		}

		files, _, ok := group.GetFiles()
		if !ok {
			return configurations, nil
		}

		wg, _ := task.WithContext(ctx)

		for _, item := range files {
			fileName := item.FileName
			wg.Go(func() error {
				configuration, err := s.load(ctx, fileName)
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

		return configurations, nil
	}
}

// load loads a single configuration item. It fetches the content from the Polaris
// server by config file name and converts it into the unified configuration
// structure. On success it also subscribes to the file's changes, so that older
// servers without the group query API can still deliver change notifications.
func (s *Source) load(ctx context.Context, file string) (*config.Configuration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	configFile, err := s.configClient.GetConfigFile(s.opts.namespace, s.opts.group, file)
	if err != nil {
		return nil, err
	}

	if !configFile.HasContent() {
		return nil, errors.New("config file not exist")
	}

	s.subscribe(file)

	configuration := conv(file, configFile.GetContent())

	return configuration, nil
}

// Store stores a configuration item.
//
// It supports only the write-only and read-write modes. It first tries
// UpsertAndPublishConfigFile to create or update and publish in one step; when the
// server does not support that API (Unimplemented) or reports a data conflict
// (DataConflict), it falls back to creating (or updating an existing) file and
// then publishing. After a successful publish the server pushes the change to
// every registered watcher.
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

	if err := s.configClient.UpsertAndPublishConfigFile(s.opts.namespace, s.opts.group, file, string(content)); err == nil {
		return nil
	} else if !isUnimplementedError(err) && !isDataConflictError(err) {
		return err
	}

	if err := s.createOrUpdateConfigFile(file, string(content)); err != nil {
		return err
	}

	return s.configClient.PublishConfigFile(s.opts.namespace, s.opts.group, file)
}

// createOrUpdateConfigFile creates or updates a config file. It first tries to
// create the file and, when creation fails because the resource already exists,
// falls back to updating it.
func (s *Source) createOrUpdateConfigFile(file, content string) error {
	if err := s.configClient.CreateConfigFile(s.opts.namespace, s.opts.group, file, content); err == nil {
		return nil
	} else if !isExistedResourceError(err) {
		return err
	}

	return s.configClient.UpdateConfigFile(s.opts.namespace, s.opts.group, file, content)
}

// Watch watches configuration items. It creates a new watcher and registers it
// with the config source; the watcher is notified when the configuration changes.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	w, err := newWatcher(ctx, s)
	if err != nil {
		return nil, err
	}

	s.watchers.Store(w, struct{}{})

	return w, nil
}

// Close closes the config source. It cancels the context and destroys the
// built-in SDK context, terminating the listen and refresh goroutines; Close
// performs the shutdown only once.
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	// Ensure the shutdown runs only once to avoid destroying the SDK context twice and panicking.
	s.once.Do(func() {
		s.cancel()

		// Wait for the listen and refresh goroutines to exit so that the SDK context is not destroyed concurrently.
		s.wg.Wait()

		if s.builtin {
			s.opts.client.Destroy()
		}
	})

	return nil
}

// listen handles listen requests. It consumes the config file names produced by
// the search loop and registers a change listener for each.
func (s *Source) listen() {
	if s.err != nil {
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case fileName, ok := <-s.chListen:
			if !ok {
				return
			}

			s.subscribe(fileName)
		case <-s.chCancel:
			// The SDK exposes no API to remove a file listener; after the config file is deleted
			// the server stops pushing changes, so the registered listener is kept as is and can
			// still receive change notifications if the file is recreated.
		}
	}
}

// subscribe subscribes to config file changes. It creates an object for the config
// file and registers a change listener, registering at most once per file. The SDK
// exposes no API to remove a listener, so a listener registered before a file is
// deleted stays valid when the file is recreated; the set of subscribed files is
// therefore recorded to avoid duplicate registration and duplicate notifications.
// The first subscription immediately pushes the current config content so that the
// watcher sees the latest published configuration (matching Nacos ListenConfig,
// which also invokes the callback once right after a listener is registered).
func (s *Source) subscribe(fileName string) {
	// Reserve the slot atomically so that only one goroutine performs the network
	// subscription and listener registration per file, avoiding duplicate listeners
	// and duplicate notifications under concurrency.
	if _, loaded := s.subscribed.LoadOrStore(fileName, struct{}{}); loaded {
		return
	}

	req := &api.GetConfigFileRequest{}
	req.Namespace = s.opts.namespace
	req.FileGroup = s.opts.group
	req.FileName = fileName
	req.Subscribe = true

	configFile, err := s.configClient.FetchConfigFile(req)
	if err != nil {
		// Remove the reservation when the subscription fails so that a later search cycle can retry it.
		s.subscribed.Delete(fileName)
		log.Warnf("%s/%s/%s listen failed: %v", s.opts.namespace, s.opts.group, fileName, err)
		return
	}

	configFile.AddChangeListener(s.onChange)

	s.notify(conv(fileName, configFile.GetContent()))
}

// notify notifies a configuration change. It notifies every registered watcher of
// the new configuration.
func (s *Source) notify(configuration *config.Configuration) {
	s.watchers.Range(func(key, value any) bool {
		w := key.(*watcher)
		w.notice(configuration)
		return true
	})
}

// refresh refreshes the configuration periodically. It runs a config search every
// 3 seconds to detect config files added to or removed from the group.
func (s *Source) refresh() {
	if s.err != nil {
		return
	}

	ticker := time.NewTicker(3 * time.Second)
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

// search searches for config files under the group. It fetches every published
// config file in the group, sends a listen request for each added file and a
// cancel request for each removed file.
func (s *Source) search() {
	if s.searchDisabled {
		return
	}

	s.version++

	group, err := s.groupClient.GetConfigGroup(s.opts.namespace, s.opts.group)
	if err != nil {
		if isUnimplementedError(err) {
			// The server does not support the group query API (GetConfigFileMetadataList);
			// disable group search and rely solely on explicit Load calls to subscribe to
			// config file changes.
			s.searchDisabled = true
			log.Warnf("search config list is not supported by server, disable group search: %v", err)
		} else {
			log.Warnf("search config list failed: %v", err)
			// Return immediately when the query fails; otherwise the incremented version
			// with an unrefreshed config list would make the cancel loop below treat every
			// item as stale and cancel all listeners in bulk.
		}
		return
	}

	files, _, ok := group.GetFiles()
	if !ok {
		// The group does not exist or has no published config, so cancel all listeners.
		for fileName := range s.versions {
			select {
			case s.chCancel <- fileName:
			case <-s.ctx.Done():
				return
			}

			delete(s.versions, fileName)
		}

		return
	}

	for _, item := range files {
		_, found := s.versions[item.FileName]
		_, subscribed := s.subscribed.Load(item.FileName)

		// Both newly discovered files and files whose previous subscription failed need
		// a (re)issued listen request.
		if !found || !subscribed {
			select {
			case s.chListen <- item.FileName:
			case <-s.ctx.Done():
				return
			}
		}

		s.versions[item.FileName] = s.version
	}

	for fileName, version := range s.versions {
		if version != s.version {
			select {
			case s.chCancel <- fileName:
			case <-s.ctx.Done():
				return
			}

			// Remove it from the version table right after cancelling so that no duplicate
			// cancel request is sent and the file can be listened to again if it is recreated.
			delete(s.versions, fileName)
		}
	}
}

// onChange is the config change callback. It is triggered when the Polaris server
// pushes a configuration change and notifies every registered watcher of the new
// configuration. When the config file is deleted (Deleted) notification is
// skipped so that no empty content is pushed to the watchers.
func (s *Source) onChange(event model.ConfigFileChangeEvent) {
	if event.ChangeType == model.Deleted {
		return
	}

	s.notify(conv(event.ConfigFileMetadata.GetFileName(), event.NewValue))
}

// buildClient builds the Polaris SDK context. It sets the server addresses, the
// communication protocol and the request timeout, then builds the SDK context
// from the configuration.
func (s *Source) buildClient() (api.SDKContext, error) {
	cfg := api.NewConfiguration()

	cfg.GetGlobal().GetServerConnector().SetAddresses(s.opts.urls)
	cfg.GetGlobal().GetServerConnector().SetProtocol(s.opts.protocol)
	cfg.GetGlobal().GetServerConnector().SetMessageTimeout(s.opts.timeout)
	cfg.GetGlobal().GetAPI().SetTimeout(s.opts.timeout)

	return api.InitContextByConfig(cfg)
}

// isUnimplementedError reports whether err is an unimplemented API error. Older
// servers may not implement UpsertAndPublishConfigFile and the gRPC layer then
// returns an Unimplemented status code. The SDK wraps that error as a
// model.SDKError whose underlying cause is not exported, so status.Code cannot be
// used directly and only the "Unimplemented" marker in the error text can be matched.
func isUnimplementedError(err error) bool {
	return strings.Contains(err.Error(), "Unimplemented")
}

// isDataConflictError reports whether err is a data conflict error. The
// UpsertAndPublishConfigFile implementation of some server versions is flawed and
// returns Code_DataConflict (409000) when the config file already exists; in that
// case the caller falls back to creating (or updating an existing) file and then
// publishing.
func isDataConflictError(err error) bool {
	e, ok := err.(model.SDKError)
	if !ok {
		return false
	}

	return strings.Contains(e.Error(), fmt.Sprintf("server code %d", apimodel.Code_DataConflict))
}

// isExistedResourceError reports whether err is an already-exists error. The server
// returns Code_ExistedResource (400201) when the config file already exists.
func isExistedResourceError(err error) bool {
	e, ok := err.(model.SDKError)
	if !ok {
		return false
	}

	return strings.Contains(e.Error(), fmt.Sprintf("server code %d", apimodel.Code_ExistedResource))
}

// conv converts a configuration. It converts a config file name and its content
// into the unified configuration structure, using the file extension of the name
// as the config format.
func conv(file, content string) *config.Configuration {
	ext := filepath.Ext(file)

	return &config.Configuration{
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  []byte(content),
		FullPath: file,
	}
}
