package polaris

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/pattern"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/polarismesh/polaris-go/api"
	plog "github.com/polarismesh/polaris-go/pkg/log"
	"github.com/polarismesh/polaris-go/pkg/model"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
)

// Name is the name of the config source.
const Name = "polaris"

// loadConcurrency limits the concurrent content fetches of one full load, so that loading a large
// group does not create an unbounded number of requests and goroutines at once.
const loadConcurrency = 8

// refreshInterval is the interval at which the config group is searched again to detect config
// files added to or removed from the group.
const refreshInterval = 3 * time.Second

// Source is a config source backed by Polaris.
type Source struct {
	err            error              // Error encountered while building the client
	opts           *options           // Config options
	ctx            context.Context    // Context
	cancel         context.CancelFunc // Cancel function
	builtin        bool               // Whether the client is built in
	version        uint64             // Current search version
	versions       map[string]uint64  // Search version of each config file
	searchFailed   bool               // Whether a search failure was logged; refresh goroutine only
	chListen       chan string        // Listen request channel
	chCancel       chan string        // Cancel-listen request channel
	watchers       sync.Map           // Watcher set
	once           sync.Once          // Ensures the shutdown runs only once
	closed         atomic.Bool        // Whether the source has been closed
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

	o.files = pattern.NormalizeFiles(o.files)

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

	s.wg.Go(s.listen)
	s.wg.Go(s.refresh)

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

	if s.closed.Load() {
		return nil, errors.ErrClosedConfigSource
	}

	if len(file) > 0 && file[0] != "" {
		if !s.matchFile(file[0]) {
			return nil, errors.ErrFileNotConfigured
		}

		configuration, err := s.load(ctx, file[0])
		if err != nil {
			return nil, err
		}

		return []*config.Configuration{configuration}, nil
	}

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

	wg, gctx := task.WithContext(ctx)
	wg.SetLimit(loadConcurrency)

	for _, item := range files {
		fileName := item.FileName

		if !s.matchFile(fileName) {
			continue
		}

		wg.Go(func() error {
			configuration, err := s.load(gctx, fileName)
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

	// Keep the result deterministic regardless of the fetch completion order.
	sort.Slice(configurations, func(i, j int) bool {
		return configurations[i].File < configurations[j].File
	})

	return configurations, nil
}

// load loads a single configuration item. It fetches the content from the Polaris
// server by config file name and converts it into the unified configuration
// structure. On success it also subscribes to the file's changes, so that older
// servers without the group query API can still deliver change notifications. A
// published config file with empty content is returned as an empty config, matching
// the other config sources.
func (s *Source) load(ctx context.Context, file string) (*config.Configuration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	configFile, err := s.configClient.GetConfigFile(s.opts.namespace, s.opts.group, file)
	if err != nil {
		return nil, err
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

	if s.closed.Load() {
		return errors.ErrClosedConfigSource
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if file == "" {
		return errors.New("invalid file name: empty")
	}

	if !s.matchFile(file) {
		return errors.ErrFileNotConfigured
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

// matchFile reports whether file is selected by the configured file list. When no
// file is configured, every file is selected.
func (s *Source) matchFile(file string) bool {
	if len(s.opts.files) == 0 {
		return true
	}

	return pattern.MatchFile(s.opts.files, file)
}

// Watch watches configuration items. It creates a new watcher and registers it
// with the config source; the watcher is notified when the configuration changes.
// It reports [errors.ErrClosedConfigSource] once the source has been closed.
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	if s.closed.Load() {
		return nil, errors.ErrClosedConfigSource
	}

	w, err := newWatcher(ctx, s)
	if err != nil {
		return nil, err
	}

	s.watchers.Store(w, struct{}{})

	// The source may have been closed between the check above and the registration, in which case
	// the shutdown no longer visits the watcher. Stop it so that it does not stay registered.
	if s.closed.Load() {
		_ = w.Stop()

		return nil, errors.ErrClosedConfigSource
	}

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
		s.closed.Store(true)
		s.cancel()

		// Wait for the listen and refresh goroutines to exit so that the SDK context is not destroyed concurrently.
		s.wg.Wait()

		// Stop the registered watchers so that their Next returns instead of blocking forever on
		// a watcher context that is not derived from the source context.
		s.watchers.Range(func(key, _ any) bool {
			_ = key.(*watcher).Stop()

			return true
		})

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
	if !s.matchFile(fileName) {
		return
	}

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

	ticker := time.NewTicker(refreshInterval)
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
		} else if !s.searchFailed {
			// Warn once per failure streak: an unreachable server would otherwise log a warning at
			// every refresh interval.
			s.searchFailed = true
			log.Warnf("search config list failed: %v", err)
		}
		// Return immediately when the query fails; otherwise the incremented version
		// with an unrefreshed config list would make the cancel loop below treat every
		// item as stale and cancel all listeners in bulk.
		return
	}

	if s.searchFailed {
		s.searchFailed = false
		log.Infof("search config list recovered")
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
		if !s.matchFile(item.FileName) {
			continue
		}

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

	fileName := event.ConfigFileMetadata.GetFileName()
	if !s.matchFile(fileName) {
		return
	}

	s.notify(conv(fileName, event.NewValue))
}

// buildClient builds the Polaris SDK context. It sets the server addresses, the
// communication protocol, the request timeout, the cache directory and the log
// directory, then builds the SDK context from the configuration.
func (s *Source) buildClient() (api.SDKContext, error) {
	cfg := api.NewConfiguration()

	cfg.GetGlobal().GetServerConnector().SetAddresses(s.opts.urls)
	cfg.GetGlobal().GetServerConnector().SetProtocol(s.opts.protocol)
	cfg.GetGlobal().GetServerConnector().SetMessageTimeout(s.opts.timeout)
	cfg.GetGlobal().GetAPI().SetTimeout(s.opts.timeout)
	cfg.GetConfigFile().GetLocalCache().SetPersistDir(s.opts.cacheDir)

	if err := setLoggersDir(s.opts.logDir); err != nil {
		return nil, errors.New("configure polaris loggers failed: " + err.Error())
	}

	return api.InitContextByConfig(cfg)
}

// setLoggersDir configures the process-wide Polaris SDK loggers once.
//
// The SDK shares its loggers across all contexts, so the built-in client only takes them over while
// they still use the SDK defaults. Loggers configured by the application or by another component
// are left untouched, and the configuration runs at most once per process to avoid rebuilding the
// file loggers and leaking the replaced ones.
var (
	setLoggersDirOnce sync.Once
	setLoggersDirErr  error
)

func setLoggersDir(logDir string) error {
	setLoggersDirOnce.Do(func() {
		if !loggersStillDefault() {
			return
		}

		setLoggersDirErr = api.SetLoggersDir(logDir)
	})

	return setLoggersDirErr
}

// loggersStillDefault reports whether every Polaris logger still uses the SDK default directory and
// log level.
func loggersStillDefault() bool {
	loggers := []plog.Logger{
		plog.GetBaseLogger(),
		plog.GetStatLogger(),
		plog.GetStatReportLogger(),
		plog.GetDetectLogger(),
		plog.GetNetworkLogger(),
		plog.GetCacheLogger(),
		plog.GetEventLogger(),
		plog.GetLosslessLogger(),
		plog.GetRouteLogger(),
		plog.GetAuthLogger(),
		plog.GetRateLimitLogger(),
		plog.GetCircuitBreakerLogger(),
	}

	for _, logger := range loggers {
		if l, ok := logger.(plog.DirLogger); ok && !isDefaultLogDir(l.GetLogDir()) {
			return false
		}

		if currentLogLevel(logger) != plog.DefaultLogLevel {
			return false
		}
	}

	return true
}

// currentLogLevel returns the lowest log level enabled by logger.
func currentLogLevel(logger plog.Logger) int {
	for _, level := range []int{plog.TraceLog, plog.DebugLog, plog.InfoLog, plog.WarnLog, plog.ErrorLog, plog.FatalLog} {
		if logger.IsLevelEnabled(level) {
			return level
		}
	}

	return plog.NoneLog
}

// isDefaultLogDir reports whether dir is the SDK default log root or one of its subdirectories.
func isDefaultLogDir(dir string) bool {
	root := filepath.Clean(plog.DefaultLogRotationRootDir)

	cleaned := filepath.Clean(dir)

	return cleaned == root || strings.HasPrefix(cleaned, root+string(filepath.Separator))
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
// as the config format. The file name is relative to the group, so it is both the
// path within the source and the file name, matching the other config sources.
func conv(file, content string) *config.Configuration {
	ext := path.Ext(file)

	return &config.Configuration{
		Path:     file,
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  []byte(content),
		FullPath: file,
	}
}
