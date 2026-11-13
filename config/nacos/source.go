package nacos

import (
	"context"
	"net"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"golang.org/x/sync/errgroup"
)

// Name is the name of the config source.
const Name = "nacos"

const (
	// loadConcurrency limits the concurrent content fetches of one config list page, so that a full
	// load does not allocate an unbounded number of requests and goroutines at once.
	loadConcurrency = 8
	// searchPageSize is the page size used when paging through the config list of a group.
	searchPageSize = 20
	// refreshInterval is the interval at which the config list of a group is searched again to
	// detect added or removed configuration items.
	refreshInterval = 10 * time.Second
)

// listenRequest is a listen request. It carries a dataId and the channel used to
// report the listen result.
type listenRequest struct {
	dataId string     // Config dataId
	done   chan error // Channel reporting the listen result
}

// cachedItem is the cached content of one config item.
type cachedItem struct {
	md5     string // Md5 reported by the config list
	content string // Config content
}

// Source is a config source backed by Nacos.
type Source struct {
	err          error                 // Error encountered while building the client
	opts         *options              // Config options
	ctx          context.Context       // Context
	cancel       context.CancelFunc    // Cancel function
	builtin      bool                  // Whether the client is built in
	version      uint64                // Current search version
	versions     map[string]uint64     // Search version of each dataId
	searchFailed bool                  // Whether a search failure was logged; refresh goroutine only
	patternMode  bool                  // Whether the file list contains "*" patterns
	chListen     chan listenRequest    // Listen request channel
	chCancel     chan string           // Cancel-listen request channel
	watchers     sync.Map              // Watcher set
	once         sync.Once             // Ensures the shutdown runs only once
	wg           sync.WaitGroup        // Wait group for goroutine exit
	cacheMu      sync.Mutex            // Guards the content cache
	cache        map[string]cachedItem // Content cache of the last full load, keyed by dataId
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

	o.files = normalizeFiles(o.files)

	s := &Source{opts: o}
	for _, file := range o.files {
		if config.IsWildcard(file) {
			s.patternMode = true
			break
		}
	}

	s.ctx, s.cancel = context.WithCancel(o.ctx)
	s.versions = make(map[string]uint64)
	s.chListen = make(chan listenRequest)
	s.chCancel = make(chan string)
	s.cache = make(map[string]cachedItem)

	if o.client == nil {
		o.client, s.err = s.buildClient()
		s.builtin = true
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
// When file is provided it loads only that item, which must be among the configured files when a
// fixed file list is set; otherwise it loads the fixed file list when one is configured, or pages
// through and loads every item under the group. On the very first full load (cache empty) Load asks
// the client to export all contents in one shot if supported, so N GetConfig calls are replaced by
// a single ZIP download. Later full loads keep the md5-reuse shortcut.
//
// A configured file entry may contain the "*" wildcard. In that case Load searches the group,
// narrowed by a server-side blur keyword, keeps only the matching items and never uses the export
// shortcut, so that the content of unmatched configurations is not loaded into memory.
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	if len(file) > 0 && file[0] != "" {
		// A configured file list also constrains an explicit load, so that this source only exposes
		// the configurations it is configured to load and listen to.
		if len(s.opts.files) > 0 && !s.matchFile(file[0]) {
			return nil, errors.ErrFileNotConfigured
		}

		configuration, err := s.load(ctx, file[0])
		if err != nil {
			return nil, err
		}

		return []*config.Configuration{configuration}, nil
	}

	if len(s.opts.files) > 0 {
		// Pattern mode searches the group and keeps only the matching items, so the export shortcut
		// and the fixed-list fast path do not apply.
		if s.patternMode {
			return s.loadPatterns(ctx)
		}

		// Fixed file list: load only the configured dataIds, without searching the group. These items
		// carry no md5, so the content cache cannot detect a change: it is neither read nor replaced.
		configurations, _, err := s.loadItems(ctx, s.fixedItems())
		if err != nil {
			return nil, err
		}

		return configurations, nil
	}

	var (
		index          = 1
		configurations = make([]*config.Configuration, 0)
		loaded         = make(map[string]cachedItem) // Content cache of this load
	)

	// Optional: one-shot export to skip N GetConfig calls on the first load.
	var exportContent map[string]string
	if s.cacheIsEmpty() {
		if ec, ok := s.opts.client.(exportClient); ok {
			if contentMap, err := ec.Export(s.opts.groupName); err == nil && len(contentMap) > 0 {
				exportContent = contentMap
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// continue
		}

		result, err := s.searchPage(index, "")
		if err != nil {
			return nil, err
		}

		if len(result.PageItems) > 0 {
			if exportContent != nil {
				// Export shortcut: content already fetched, skip GetConfig per item.
				for _, item := range result.PageItems {
					content := exportContent[item.DataId]
					if content == "" {
						// The export missed this item; fetch it individually.
						c, err := s.fetchContent(ctx, item.DataId)
						if err != nil {
							return nil, err
						}
						content = c
					}

					configurations = append(configurations, s.conv(item.DataId, content))
					loaded[item.DataId] = cachedItem{md5: item.Md5, content: content}
				}
			} else {
				configs, entries, err := s.loadItems(ctx, result.PageItems)
				if err != nil {
					return nil, err
				}

				configurations = append(configurations, configs...)

				for dataId, entry := range entries {
					loaded[dataId] = entry
				}
			}
		}

		if result.PageNumber >= result.PagesAvailable {
			break
		}

		// Advance locally and stop when the response does not move forward, so that a page that
		// does not echo the requested number cannot keep the loop on the same page forever.
		next := result.PageNumber + 1
		if next <= index {
			break
		}

		index = next
	}

	// Replace the cache with the items of this load, so that removed items do not stay cached.
	s.replaceCache(loaded)

	return configurations, nil
}

// load loads a single configuration item. It fetches the content from the Nacos
// server by dataId and converts it into the unified configuration structure.
func (s *Source) load(ctx context.Context, file string) (*config.Configuration, error) {
	content, err := s.fetchContent(ctx, file)
	if err != nil {
		return nil, err
	}

	return s.conv(file, content), nil
}

// matchFile reports whether dataId is selected by the configured file list, which may mix exact
// dataIds and "*" patterns.
func (s *Source) matchFile(dataId string) bool {
	return config.MatchFile(s.opts.files, dataId)
}

// fixedItems converts an exact file list into config list items, so that loading reuses the same
// per-item logic as a full group load. The items carry no md5, so the content of every listed
// dataId is fetched and the export shortcut is skipped. Pattern mode does not use it.
func (s *Source) fixedItems() []model.ConfigItem {
	items := make([]model.ConfigItem, 0, len(s.opts.files))
	for _, file := range s.opts.files {
		items = append(items, model.ConfigItem{DataId: file})
	}

	return items
}

// loadPatterns loads the configurations selected by the configured patterns. It searches the group
// instead of using the export shortcut, so that only the content of matching items is fetched.
func (s *Source) loadPatterns(ctx context.Context) ([]*config.Configuration, error) {
	items, err := s.discover(ctx)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, nil
	}

	configurations, entries, err := s.loadItems(ctx, items)
	if err != nil {
		return nil, err
	}

	s.replaceCache(entries)

	// Keep the result deterministic regardless of the fetch completion order.
	sort.Slice(configurations, func(i, j int) bool {
		return configurations[i].File < configurations[j].File
	})

	return configurations, nil
}

// discover returns the config list items selected by the configured files. A group with no file
// list yields every item. With a pattern list the candidate set is narrowed by one server-side blur
// keyword per pattern and then filtered by the configured files; exact entries are appended when
// the search does not return them. The result is deduplicated and sorted by dataId.
func (s *Source) discover(ctx context.Context) ([]model.ConfigItem, error) {
	var (
		terms = s.searchTerms()
		full  = len(s.opts.files) == 0 || terms == nil || s.hasUntetheredPattern()
		items = make([]model.ConfigItem, 0)
		seen  = make(map[string]struct{})
	)

	collect := func(pageItems []model.ConfigItem) {
		for i := range pageItems {
			item := pageItems[i]
			if _, ok := seen[item.DataId]; ok {
				continue
			}
			if len(s.opts.files) > 0 && !s.matchFile(item.DataId) {
				continue
			}

			seen[item.DataId] = struct{}{}
			items = append(items, item)
		}
	}

	// A whole-group scan already covers every term, so it is used alone when a pattern has no
	// usable keyword. Otherwise each keyword narrows the server-side search, which is what keeps
	// the candidate set out of memory.
	if full {
		if err := s.eachSearchPage(ctx, "", collect); err != nil {
			return nil, err
		}
	} else {
		for _, term := range terms {
			if err := s.eachSearchPage(ctx, term, collect); err != nil {
				return nil, err
			}
		}
	}

	// An exact entry does not necessarily contain any search term, so add the missing ones. The
	// synthetic item carries no md5 and its content is fetched individually.
	for _, file := range s.opts.files {
		if config.IsWildcard(file) {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}

		seen[file] = struct{}{}
		items = append(items, model.ConfigItem{DataId: file})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].DataId < items[j].DataId
	})

	return items, nil
}

// searchTerms returns the deduplicated server-side blur keywords of the configured patterns. It
// returns nil when no pattern has a literal segment long enough to narrow the search, in which case
// the caller scans the whole group once.
func (s *Source) searchTerms() []string {
	var (
		seen  = make(map[string]struct{}, len(s.opts.files))
		terms = make([]string, 0, len(s.opts.files))
	)

	for _, file := range s.opts.files {
		if !config.IsWildcard(file) {
			continue
		}

		term := patternTerm(file)
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}

		seen[term] = struct{}{}
		terms = append(terms, term)
	}

	if len(terms) == 0 {
		return nil
	}

	return terms
}

// hasUntetheredPattern reports whether a configured pattern has no literal segment usable as a
// server-side keyword, so that the group has to be scanned once for it.
func (s *Source) hasUntetheredPattern() bool {
	for _, file := range s.opts.files {
		if config.IsWildcard(file) && patternTerm(file) == "" {
			return true
		}
	}

	return false
}

// eachSearchPage pages through the config list of the group and calls fn for every page. dataId is
// used as the server-side blur keyword; an empty value lists the whole group.
func (s *Source) eachSearchPage(ctx context.Context, dataId string, fn func([]model.ConfigItem)) error {
	index := 1

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// continue
		}

		result, err := s.searchPage(index, dataId)
		if err != nil {
			return err
		}

		if len(result.PageItems) > 0 {
			fn(result.PageItems)
		}

		if result.PageNumber >= result.PagesAvailable {
			return nil
		}

		// Advance locally and stop when the response does not move forward, so that a page that
		// does not echo the requested number cannot keep the loop on the same page forever.
		next := result.PageNumber + 1
		if next <= index {
			return nil
		}

		index = next
	}
}

// loadItems loads one config list page. It reuses the cached content of an unchanged item and
// fetches the content of a new or changed one, with a bounded number of concurrent requests. The
// first failure cancels the remaining fetches of the page.
func (s *Source) loadItems(ctx context.Context, items []model.ConfigItem) ([]*config.Configuration, map[string]cachedItem, error) {
	var (
		mu      sync.Mutex
		configs = make([]*config.Configuration, 0, len(items))
		entries = make(map[string]cachedItem, len(items))
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(loadConcurrency)

	for _, item := range items {
		g.Go(func() error {
			content, ok := s.cachedContent(item)
			if !ok {
				fetched, err := s.fetchContent(gctx, item.DataId)
				if err != nil {
					return err
				}

				content = fetched
			}

			mu.Lock()
			configs = append(configs, s.conv(item.DataId, content))
			if item.Md5 != "" {
				// An entry without md5 can never be reused, so it is not recorded.
				entries[item.DataId] = cachedItem{md5: item.Md5, content: content}
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	return configs, entries, nil
}

// cachedContent returns the content of an item that does not need to be fetched again. The content
// of an unchanged item is reused from the previous load, and a page that carries the content itself
// is used as is.
func (s *Source) cachedContent(item model.ConfigItem) (string, bool) {
	if s.opts.openKMS {
		// Decryption has to run on the content returned by the server.
		return "", false
	}

	if item.Content != "" {
		return item.Content, true
	}

	if item.Md5 == "" {
		return "", false
	}

	s.cacheMu.Lock()
	entry, ok := s.cache[item.DataId]
	s.cacheMu.Unlock()

	if ok && entry.md5 == item.Md5 && entry.content != "" {
		return entry.content, true
	}

	return "", false
}

// fetchContent fetches the content of one dataId from the Nacos server.
func (s *Source) fetchContent(ctx context.Context, file string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	return s.opts.client.GetConfig(vo.ConfigParam{
		DataId: file,
		Group:  s.opts.groupName,
	})
}

// replaceCache swaps the content cache with the items of the last full load.
func (s *Source) replaceCache(items map[string]cachedItem) {
	s.cacheMu.Lock()
	s.cache = items
	s.cacheMu.Unlock()
}

// invalidateCache drops the cached content of an item whose content changed.
func (s *Source) invalidateCache(file string) {
	s.cacheMu.Lock()
	delete(s.cache, file)
	s.cacheMu.Unlock()
}

// cacheIsEmpty reports whether the content cache has no entry. It is used to decide whether the
// export shortcut can substitute N GetConfig calls during the very first full load.
func (s *Source) cacheIsEmpty() bool {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	return len(s.cache) == 0
}

// exportClient is implemented by config clients that can batch-export all configurations at once,
// bypassing N individual GetConfig requests on the first full load. listClient implements it; the
// fallback SDK client does not, so Load degrades to per-config fetch transparently.
type exportClient interface {
	Export(group string) (map[string]string, error)
}

// Store stores a configuration item.
//
// It supports only the write-only and read-write modes. After a successful
// publish the server pushes the change to every registered watcher. When a file
// list is configured, only a file selected by it can be stored.
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	// A configured file list also constrains an explicit store, so that this source only writes the
	// configurations it is configured to manage.
	if len(s.opts.files) > 0 && !s.matchFile(file) {
		return errors.ErrFileNotConfigured
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

	// The source may have been closed between the context check above and the registration, in
	// which case the shutdown no longer visits the watcher. Stop it so that Next does not block
	// forever on a closed source.
	if err := s.ctx.Err(); err != nil {
		_ = w.Stop()

		return nil, err
	}

	return w, nil
}

// Close closes the config source. It cancels the context, terminates the listen and
// refresh goroutines, stops the registered watchers and closes the built-in client;
// Close performs the shutdown only once.
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	// Ensure the shutdown runs only once to avoid closing the client twice and panicking.
	s.once.Do(func() {
		s.cancel()

		// Wait for the listen and refresh goroutines to exit so that the client is not closed concurrently.
		s.wg.Wait()

		// Stop the registered watchers so that their Next returns instead of blocking forever on
		// a watcher context that is not derived from the source context.
		s.watchers.Range(func(key, _ any) bool {
			_ = key.(*watcher).Stop()

			return true
		})

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

// refresh registers the configuration listening. When an exact file list is configured, it
// registers those dataIds once because the list never changes; otherwise, including pattern mode,
// it runs a config search at a fixed interval to detect added or removed configuration items.
func (s *Source) refresh() {
	if s.err != nil {
		return
	}

	if len(s.opts.files) > 0 && !s.patternMode {
		s.listenFiles()
		return
	}

	s.search()

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

// listenFiles registers config listening for the fixed file list. A failed registration is logged
// by the listen goroutine and not retried, since the list is fixed and the error is deterministic.
func (s *Source) listenFiles() {
	for _, dataId := range s.opts.files {
		select {
		case <-s.ctx.Done():
			return
		default:
			// continue
		}

		_ = s.requestListen(dataId)
	}
}

// searchPage requests one page of the config list of the group. Loading and the periodic search
// share it so that both page through the same view with the same page size. dataId narrows the
// search with the server-side blur keyword; an empty value lists the whole group.
func (s *Source) searchPage(pageNo int, dataId string) (*model.ConfigPage, error) {
	return s.opts.client.SearchConfig(vo.SearchConfigParam{
		Search:   "blur",
		DataId:   dataId,
		Group:    s.opts.groupName,
		PageNo:   pageNo,
		PageSize: searchPageSize,
	})
}

// search searches for configuration items under the group. A group without a file list is paged
// and handled page by page, so the whole group is never accumulated in memory; pattern mode
// discovers the matched items first, since they have to be merged across keywords and
// de-duplicated. It sends a listen request for each added dataId and a cancel request for each
// removed dataId. A failed search leaves the version table untouched, so a broken server does not
// cancel and then re-register every listener.
func (s *Source) search() {
	s.version++
	version := s.version

	// mark registers listening for the items and stamps them with the current version.
	mark := func(items []model.ConfigItem) {
		for _, item := range items {
			if _, ok := s.versions[item.DataId]; !ok {
				// Skip the version mark when listening fails so that the next search retries it.
				if err := s.requestListen(item.DataId); err != nil {
					continue
				}
			}

			s.versions[item.DataId] = version
		}
	}

	var err error
	if len(s.opts.files) == 0 {
		err = s.eachSearchPage(s.ctx, "", mark)
	} else {
		var items []model.ConfigItem
		if items, err = s.discover(s.ctx); err == nil {
			mark(items)
		}
	}

	if err != nil {
		if s.ctx.Err() != nil {
			return
		}

		// Warn once per failure streak: an unreachable server would otherwise log a warning at
		// every refresh interval.
		if !s.searchFailed {
			s.searchFailed = true
			log.Warnf("search config list failed: %v", err)
		}

		return
	}

	if s.searchFailed {
		s.searchFailed = false
		log.Infof("search config list recovered")
	}

	for dataId, v := range s.versions {
		if v != version {
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
	// The cached content is stale now; the next full load fetches it again.
	s.invalidateCache(file)

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
	serverConfigs, _, err := newServerConfigs(s.opts.urls)

	if len(serverConfigs) == 0 {
		if err != nil {
			return nil, err
		}

		return nil, errors.New("invalid server urls")
	}

	clientConfig := newClientConfig(s.opts)
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ServerConfigs: serverConfigs,
		ClientConfig:  &clientConfig,
	})
	if err != nil {
		return nil, err
	}

	return newListConfigClient(client, s.opts), nil
}

// newServerConfigs parses the server address list. An address may carry an optional scheme,
// defaulting to http. An invalid address is logged and skipped so that the remaining addresses
// stay usable; the returned endpoint and error describe the first invalid address, if any.
func newServerConfigs(urls []string) ([]constant.ServerConfig, string, error) {
	configs := make([]constant.ServerConfig, 0, len(urls))

	var (
		firstErr    error
		firstBadURL string
	)

	for _, v := range urls {
		serverConfig, err := parseServerConfig(v)
		if err != nil {
			log.Warnf("%s parse failed: %v", v, err)
			if firstErr == nil {
				firstErr, firstBadURL = err, v
			}
			continue
		}

		configs = append(configs, serverConfig)
	}

	return configs, firstBadURL, firstErr
}

// parseServerConfig parses one server address, prepending http when no scheme is present.
func parseServerConfig(addr string) (constant.ServerConfig, error) {
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}

	raw, err := url.Parse(addr)
	if err != nil {
		return constant.ServerConfig{}, err
	}

	host, p, err := net.SplitHostPort(raw.Host)
	if err != nil {
		return constant.ServerConfig{}, err
	}

	port, err := strconv.ParseUint(p, 10, 64)
	if err != nil {
		return constant.ServerConfig{}, err
	}

	return constant.ServerConfig{
		Scheme:      raw.Scheme,
		ContextPath: raw.Path,
		IpAddr:      host,
		Port:        port,
	}, nil
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
// config format. The dataId is relative to the group, so it is both the path within
// the source and the file name, matching the other config sources.
func (s *Source) conv(file, content string) *config.Configuration {
	ext := filepath.Ext(file)

	return &config.Configuration{
		Path:     file,
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  []byte(content),
		FullPath: file,
	}
}
