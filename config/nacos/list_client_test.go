package nacos

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/common/nacos_error"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var _ config_client.IConfigClient = (*stubConfigClient)(nil)

// stubConfigClient is a config client whose SearchConfig reports how often the fallback is used.
type stubConfigClient struct {
	page        *model.ConfigPage
	searchErr   error
	searchFunc  func(vo.SearchConfigParam) (*model.ConfigPage, error)
	calls       atomic.Int32
	getCalls    atomic.Int32
	listenCalls atomic.Int32
}

func (c *stubConfigClient) GetConfig(vo.ConfigParam) (string, error) {
	c.getCalls.Add(1)

	return "fetched", nil
}

func (c *stubConfigClient) PublishConfig(vo.ConfigParam) (bool, error) {
	return true, nil
}

func (c *stubConfigClient) DeleteConfig(vo.ConfigParam) (bool, error) {
	return true, nil
}

func (c *stubConfigClient) ListenConfig(vo.ConfigParam) error {
	c.listenCalls.Add(1)

	return nil
}

func (c *stubConfigClient) CancelListenConfig(vo.ConfigParam) error {
	return nil
}

func (c *stubConfigClient) SearchConfig(param vo.SearchConfigParam) (*model.ConfigPage, error) {
	c.calls.Add(1)

	if c.searchFunc != nil {
		return c.searchFunc(param)
	}

	if c.searchErr != nil {
		return nil, c.searchErr
	}

	if c.page == nil {
		return &model.ConfigPage{}, nil
	}

	return c.page, nil
}

func (c *stubConfigClient) CloseClient() {}

// TestListClientSearchConfig verifies that SearchConfig uses the v3 admin list API and maps the
// returned metadata without relying on the configuration content.
func TestListClientSearchConfig(t *testing.T) {
	queries := make(chan url.Values, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos"+configListPath {
			http.NotFound(w, r)

			return
		}

		queries <- r.URL.Query()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"totalCount":2,"pageNumber":2,"pagesAvailable":3,"pageItems":[{"dataId":"a.json","groupName":"test-group","namespaceId":"test-ns","md5":"m1","content":"ignored"},{"dataId":"b.yaml","groupName":"test-group","namespaceId":"test-ns","md5":"m2"}]}}`))
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.namespaceId = "test-ns"
	o.timeout = time.Second

	stub := &stubConfigClient{}
	client := newListConfigClient(stub, o)

	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client, got %T", client)
	}

	page, err := list.SearchConfig(vo.SearchConfigParam{
		Search:   "blur",
		Group:    "test-group",
		PageNo:   2,
		PageSize: 50,
	})
	if err != nil {
		t.Fatalf("search config failed: %v", err)
	}

	query := <-queries
	if got := query.Get("groupName"); got != "test-group" {
		t.Fatalf("expect groupName test-group, got %q", got)
	}
	if got := query.Get("namespaceId"); got != "test-ns" {
		t.Fatalf("expect namespaceId test-ns, got %q", got)
	}
	if got := query.Get("pageNo"); got != "2" {
		t.Fatalf("expect pageNo 2, got %q", got)
	}
	if got := query.Get("pageSize"); got != "50" {
		t.Fatalf("expect pageSize 50, got %q", got)
	}
	if got := query.Get("search"); got != "blur" {
		t.Fatalf("expect search blur, got %q", got)
	}

	if page.TotalCount != 2 || page.PageNumber != 2 || page.PagesAvailable != 3 {
		t.Fatalf("unexpected page metadata: %+v", page)
	}
	if len(page.PageItems) != 2 {
		t.Fatalf("expect 2 page items, got %d", len(page.PageItems))
	}
	if page.PageItems[0].DataId != "a.json" || page.PageItems[1].DataId != "b.yaml" {
		t.Fatalf("unexpected page items: %+v", page.PageItems)
	}
	if page.PageItems[0].Content != "" {
		t.Fatalf("the list response must not carry config content, got %q", page.PageItems[0].Content)
	}

	if calls := stub.calls.Load(); calls != 0 {
		t.Fatalf("expect the wrapped client not to be used, got %d calls", calls)
	}

	list.CloseClient()
}

// TestListClientFallback verifies that an unsupported v3 endpoint falls back to the wrapped client
// and disables the v3 path for later searches.
func TestListClientFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.timeout = time.Second

	stub := &stubConfigClient{page: &model.ConfigPage{
		TotalCount: 1,
		PageItems:  []model.ConfigItem{{DataId: "fallback.json"}},
	}}
	client := newListConfigClient(stub, o)

	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client, got %T", client)
	}

	page, err := list.SearchConfig(vo.SearchConfigParam{Search: "blur"})
	if err != nil {
		t.Fatalf("search config failed: %v", err)
	}
	if len(page.PageItems) != 1 || page.PageItems[0].DataId != "fallback.json" {
		t.Fatalf("expect the fallback page, got %+v", page)
	}
	if !list.disabled.Load() {
		t.Fatal("the v3 list path should be disabled after an unsupported response")
	}

	if _, err = list.SearchConfig(vo.SearchConfigParam{Search: "blur"}); err != nil {
		t.Fatalf("second search failed: %v", err)
	}
	if calls := stub.calls.Load(); calls != 2 {
		t.Fatalf("expect the fallback client to serve both searches, got %d calls", calls)
	}

	list.CloseClient()
}

// TestListClientPermissionFallback verifies that a permission failure disables the v3 path instead
// of retrying it on every search.
func TestListClientPermissionFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.username = "nacos"
	o.password = "nacos"
	o.timeout = time.Second

	stub := &stubConfigClient{page: &model.ConfigPage{
		PageItems: []model.ConfigItem{{DataId: "fallback.json"}},
	}}
	client := newListConfigClient(stub, o)

	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client, got %T", client)
	}

	if _, err := list.SearchConfig(vo.SearchConfigParam{Search: "blur"}); err != nil {
		t.Fatalf("search config failed: %v", err)
	}
	if !list.disabled.Load() {
		t.Fatal("the v3 list path should be disabled after a permission failure")
	}
	if calls := stub.calls.Load(); calls != 1 {
		t.Fatalf("expect the fallback client to serve the search, got %d calls", calls)
	}

	list.CloseClient()
}

// TestListClientUnauthorizedKeepsV3 verifies that a 401 keeps the v3 path enabled, since an
// expired token or a temporarily unavailable login endpoint may recover, instead of disabling the
// v3 path for the rest of the process lifetime.
func TestListClientUnauthorizedKeepsV3(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.username = "nacos"
	o.password = "nacos"
	o.timeout = time.Second

	stub := &stubConfigClient{page: &model.ConfigPage{
		PageItems: []model.ConfigItem{{DataId: "fallback.json"}},
	}}
	client := newListConfigClient(stub, o)

	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client, got %T", client)
	}

	page, err := list.SearchConfig(vo.SearchConfigParam{Search: "blur"})
	if err != nil {
		t.Fatalf("search config failed: %v", err)
	}
	if len(page.PageItems) != 1 || page.PageItems[0].DataId != "fallback.json" {
		t.Fatalf("expect the fallback page, got %+v", page)
	}
	if list.disabled.Load() {
		t.Fatal("a 401 should not permanently disable the v3 list path")
	}
	if calls := stub.calls.Load(); calls != 1 {
		t.Fatalf("expect the fallback client to serve the search, got %d calls", calls)
	}

	list.CloseClient()
}

// TestListClientLoginErrorKeepsStatus verifies that a failed login reports the HTTP status and a
// bounded body instead of an unbounded plain error.
func TestListClientLoginErrorKeepsStatus(t *testing.T) {
	body := bytes.Repeat([]byte("x"), maxErrorBodyLength*2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.username = "nacos"
	o.password = "nacos"
	o.timeout = time.Second

	list, ok := newListConfigClient(&stubConfigClient{}, o).(*listClient)
	if !ok {
		t.Fatal("expect a list client")
	}

	_, _, err := list.loginAt(list.servers[0])
	if err == nil {
		t.Fatal("expect a login error")
	}

	var nacosErr *nacos_error.NacosError
	if !errors.As(err, &nacosErr) {
		t.Fatalf("expect a nacos error, got %T", err)
	}
	if got := nacosErr.ErrorCode(); got != "401" {
		t.Fatalf("expect error code 401, got %q", got)
	}
	if got, want := len(nacosErr.Error()), maxErrorBodyLength+len("[401] "); got > want {
		t.Fatalf("expect an error message of at most %d bytes, got %d", want, got)
	}
}

// TestListClientInvalidURL verifies that an invalid address does not prevent the v3 path from being
// used with the remaining valid addresses.
func TestListClientInvalidURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"totalCount":1,"pageNumber":1,"pagesAvailable":1,"pageItems":[{"dataId":"valid.json","groupName":"test-group"}]}}`))
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{"bad::addr", server.URL + "/nacos"}
	o.timeout = time.Second

	stub := &stubConfigClient{}
	client := newListConfigClient(stub, o)

	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client despite an invalid url, got %T", client)
	}

	page, err := list.SearchConfig(vo.SearchConfigParam{Search: "blur", Group: "test-group"})
	if err != nil {
		t.Fatalf("search config failed: %v", err)
	}
	if len(page.PageItems) != 1 || page.PageItems[0].DataId != "valid.json" {
		t.Fatalf("unexpected page items: %+v", page.PageItems)
	}
	if calls := stub.calls.Load(); calls != 0 {
		t.Fatalf("expect the wrapped client not to be used, got %d calls", calls)
	}

	list.CloseClient()
}

// TestListClientBusinessErrorFallback verifies that a permanent v3 result code disables the v3 path,
// while a transient result code keeps it enabled for the next search.
func TestListClientBusinessErrorFallback(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		disabled bool
	}{
		{name: "access denied", code: 10001, disabled: true},
		{name: "server error", code: 30000, disabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":` + strconv.Itoa(tt.code) + `,"message":"list failed"}`))
			}))
			defer server.Close()

			o := defaultOptions()
			o.urls = []string{server.URL + "/nacos"}
			o.timeout = time.Second

			stub := &stubConfigClient{page: &model.ConfigPage{
				PageItems: []model.ConfigItem{{DataId: "fallback.json"}},
			}}
			client := newListConfigClient(stub, o)

			list, ok := client.(*listClient)
			if !ok {
				t.Fatalf("expect a list client, got %T", client)
			}

			page, err := list.SearchConfig(vo.SearchConfigParam{Search: "blur"})
			if err != nil {
				t.Fatalf("search config failed: %v", err)
			}
			if len(page.PageItems) != 1 || page.PageItems[0].DataId != "fallback.json" {
				t.Fatalf("expect the fallback page, got %+v", page)
			}
			if got := list.disabled.Load(); got != tt.disabled {
				t.Fatalf("expect disabled=%v, got %v", tt.disabled, got)
			}
			if calls := stub.calls.Load(); calls != 1 {
				t.Fatalf("expect the fallback client to serve the search, got %d calls", calls)
			}

			list.CloseClient()
		})
	}
}

// TestListClientAccurateSearch verifies that a non-blur search is delegated to the wrapped client.
func TestListClientAccurateSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.timeout = time.Second

	stub := &stubConfigClient{}
	client := newListConfigClient(stub, o)

	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client, got %T", client)
	}

	if _, err := list.SearchConfig(vo.SearchConfigParam{Search: "accurate", DataId: "a.json"}); err != nil {
		t.Fatalf("search config failed: %v", err)
	}
	if calls := stub.calls.Load(); calls != 1 {
		t.Fatalf("expect accurate search to be delegated, got %d calls", calls)
	}

	list.CloseClient()
}

// TestNewListConfigClientNil verifies that a nil wrapped client returns nil.
func TestNewListConfigClientNil(t *testing.T) {
	if client := NewListConfigClient(nil); client != nil {
		t.Fatalf("expect nil for a nil client, got %T", client)
	}
}

// TestListClientDoubleWrap verifies that a list client is not wrapped again.
func TestListClientDoubleWrap(t *testing.T) {
	inner := &listClient{}

	if client := newListConfigClient(inner, nil); client != inner {
		t.Fatalf("expect the existing list client, got %T", client)
	}
}

// TestServerAddress verifies that the context path and scheme are normalized before being joined
// with the API path.
func TestServerAddress(t *testing.T) {
	tests := []struct {
		name string
		cfg  constant.ServerConfig
		want string
	}{
		{
			name: "context path with leading and trailing slashes",
			cfg:  constant.ServerConfig{Scheme: "https", IpAddr: "10.0.0.1", Port: 443, ContextPath: "/nacos/"},
			want: "https://10.0.0.1:443/nacos" + configListPath,
		},
		{
			name: "context path without slashes",
			cfg:  constant.ServerConfig{Scheme: "http", IpAddr: "127.0.0.1", Port: 8848, ContextPath: "nacos"},
			want: "http://127.0.0.1:8848/nacos" + configListPath,
		},
		{
			name: "root context path",
			cfg:  constant.ServerConfig{Scheme: "http", IpAddr: "127.0.0.1", Port: 8848, ContextPath: "/"},
			want: "http://127.0.0.1:8848" + configListPath,
		},
		{
			name: "empty context path falls back to the default scheme and web context",
			cfg:  constant.ServerConfig{IpAddr: "127.0.0.1", Port: 8848},
			want: "http://127.0.0.1:8848/nacos" + configListPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverAddress(tt.cfg, configListPath); got != tt.want {
				t.Errorf("expect %q, got %q", tt.want, got)
			}
		})
	}
}

// TestNewServerConfigs verifies that an invalid address is skipped while the remaining addresses
// stay usable, and that the first failure is reported.
func TestNewServerConfigs(t *testing.T) {
	configs, endpoint, err := newServerConfigs([]string{
		"bad::addr",
		"10.0.0.1:8848",
		"https://10.0.0.2:443/nacos",
		"bad::addr2",
	})

	if err == nil {
		t.Fatal("expect an error for the invalid addresses")
	}
	if endpoint != "bad::addr" {
		t.Fatalf("expect the first invalid address bad::addr, got %q", endpoint)
	}
	if len(configs) != 2 {
		t.Fatalf("expect 2 valid server configs, got %d", len(configs))
	}
	if configs[0].Scheme != "http" || configs[0].IpAddr != "10.0.0.1" || configs[0].Port != 8848 {
		t.Fatalf("unexpected default scheme server config: %+v", configs[0])
	}
	if configs[1].Scheme != "https" || configs[1].ContextPath != "/nacos" {
		t.Fatalf("unexpected https server config: %+v", configs[1])
	}
}

// TestSourceLoadUsesSearchContent verifies that Load reuses the content returned by the search and
// still fetches the content when KMS decryption is required.
func TestSourceLoadUsesSearchContent(t *testing.T) {
	tests := []struct {
		name         string
		openKMS      bool
		wantContent  string
		wantGetCalls int32
	}{
		{name: "reuse search content", wantContent: "from-search", wantGetCalls: 0},
		{name: "fetch content for kms", openKMS: true, wantContent: "fetched", wantGetCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubConfigClient{page: &model.ConfigPage{
				TotalCount:     1,
				PageNumber:     1,
				PagesAvailable: 1,
				PageItems:      []model.ConfigItem{{DataId: "config.json", Content: "from-search"}},
			}}

			o := defaultOptions()
			o.client = stub
			o.groupName = "test-group"
			o.openKMS = tt.openKMS

			source := &Source{opts: o}

			configs, err := source.Load(context.Background())
			if err != nil {
				t.Fatalf("load configs failed: %v", err)
			}
			if len(configs) != 1 {
				t.Fatalf("expect one configuration, got %d", len(configs))
			}
			if got := string(configs[0].Content); got != tt.wantContent {
				t.Fatalf("expect content %q, got %q", tt.wantContent, got)
			}
			if got := stub.getCalls.Load(); got != tt.wantGetCalls {
				t.Fatalf("expect %d get config calls, got %d", tt.wantGetCalls, got)
			}
		})
	}
}

// TestSourceLoadReusesCachedContent verifies that a full load only fetches the content of a new or
// changed dataId and reuses the cached content of an unchanged one.
func TestSourceLoadReusesCachedContent(t *testing.T) {
	stub := &stubConfigClient{}

	o := defaultOptions()
	o.client = stub
	o.groupName = "test-group"

	source := &Source{opts: o}

	load := func(items ...model.ConfigItem) {
		t.Helper()

		stub.page = &model.ConfigPage{
			TotalCount:     len(items),
			PageNumber:     1,
			PagesAvailable: 1,
			PageItems:      items,
		}

		if _, err := source.Load(context.Background()); err != nil {
			t.Fatalf("load configs failed: %v", err)
		}
	}

	load(
		model.ConfigItem{DataId: "a.json", Md5: "m1"},
		model.ConfigItem{DataId: "b.yaml", Md5: "m2"},
	)
	if got := stub.getCalls.Load(); got != 2 {
		t.Fatalf("expect 2 get config calls on the first load, got %d", got)
	}

	load(
		model.ConfigItem{DataId: "a.json", Md5: "m1"},
		model.ConfigItem{DataId: "b.yaml", Md5: "m2"},
	)
	if got := stub.getCalls.Load(); got != 2 {
		t.Fatalf("expect the cached content to be reused, got %d get config calls", got)
	}

	// A changed md5 has to fetch the content again, while the unchanged item stays cached.
	load(
		model.ConfigItem{DataId: "a.json", Md5: "m1"},
		model.ConfigItem{DataId: "b.yaml", Md5: "m3"},
	)
	if got := stub.getCalls.Load(); got != 3 {
		t.Fatalf("expect 3 get config calls after the change, got %d", got)
	}

	// A pushed change invalidates the cached content of that dataId.
	source.onChange("", "", "a.json", "changed")

	load(
		model.ConfigItem{DataId: "a.json", Md5: "m1"},
		model.ConfigItem{DataId: "b.yaml", Md5: "m3"},
	)
	if got := stub.getCalls.Load(); got != 4 {
		t.Fatalf("expect 4 get config calls after the invalidation, got %d", got)
	}
}

// TestSourceLoadRespectsFixedFiles verifies that an explicit load is constrained by the configured
// file list.
func TestSourceLoadRespectsFixedFiles(t *testing.T) {
	stub := &stubConfigClient{}

	o := defaultOptions()
	o.client = stub
	o.groupName = "test-group"
	o.files = []string{"a.json"}

	source := &Source{opts: o}

	if _, err := source.Load(context.Background(), "b.yaml"); err == nil {
		t.Fatal("expect an error for a file outside the configured files")
	}

	configs, err := source.Load(context.Background(), "a.json")
	if err != nil {
		t.Fatalf("load the configured file failed: %v", err)
	}
	if len(configs) != 1 || configs[0].File != "a.json" {
		t.Fatalf("unexpected configurations: %+v", configs)
	}

	// Without an explicit file the configured list is still loaded.
	configs, err = source.Load(context.Background())
	if err != nil {
		t.Fatalf("load the fixed files failed: %v", err)
	}
	if len(configs) != 1 || configs[0].File != "a.json" {
		t.Fatalf("unexpected configurations: %+v", configs)
	}
}

// TestSourceStoreRespectsFixedFiles verifies that an explicit store is constrained by the configured
// file list.
func TestSourceStoreRespectsFixedFiles(t *testing.T) {
	stub := &stubConfigClient{}

	o := defaultOptions()
	o.client = stub
	o.groupName = "test-group"
	o.files = []string{"a.json"}
	o.mode = config.ReadWrite

	source := &Source{opts: o}

	if err := source.Store(context.Background(), "b.yaml", []byte("{}")); err == nil {
		t.Fatal("expect an error for a file outside the configured files")
	}

	if err := source.Store(context.Background(), "a.json", []byte("{}")); err != nil {
		t.Fatalf("store the configured file failed: %v", err)
	}
}

// TestSourceLoadFixedFiles verifies that a configured file list limits a full load to those
// dataIds and skips the group search.
func TestSourceLoadFixedFiles(t *testing.T) {
	stub := &stubConfigClient{
		page: &model.ConfigPage{
			TotalCount:     1,
			PageNumber:     1,
			PagesAvailable: 1,
			PageItems:      []model.ConfigItem{{DataId: "c.json", Content: "from-search"}},
		},
	}

	o := defaultOptions()
	o.client = stub
	o.groupName = "test-group"
	o.files = []string{"a.json", "b.yaml"}

	source := &Source{opts: o}

	configs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load configs failed: %v", err)
	}

	files := make(map[string]struct{}, len(configs))
	for _, configuration := range configs {
		files[configuration.File] = struct{}{}
	}
	if len(files) != 2 {
		t.Fatalf("expect 2 configurations, got %d", len(files))
	}
	if _, ok := files["a.json"]; !ok {
		t.Fatalf("expect a.json to be loaded")
	}
	if _, ok := files["b.yaml"]; !ok {
		t.Fatalf("expect b.yaml to be loaded")
	}
	if _, ok := files["c.json"]; ok {
		t.Fatalf("expect c.json to be skipped")
	}
	if got := stub.getCalls.Load(); got != 2 {
		t.Fatalf("expect 2 get config calls, got %d", got)
	}
	if got := stub.calls.Load(); got != 0 {
		t.Fatalf("expect no search calls, got %d", got)
	}
	if !source.cacheIsEmpty() {
		t.Fatal("the fixed file list must not populate the content cache")
	}
}

// TestSourceConv verifies that a dataId is exposed as the path within the source as well as the
// file name, matching the configuration structure produced by the other config sources.
func TestSourceConv(t *testing.T) {
	configuration := (&Source{}).conv("config.json", "content")

	if configuration.Path != "config.json" {
		t.Fatalf("expect path config.json, got %q", configuration.Path)
	}
	if configuration.File != "config.json" || configuration.Name != "config" {
		t.Fatalf("unexpected file or name: %+v", configuration)
	}
	if configuration.Format != "json" || string(configuration.Content) != "content" {
		t.Fatalf("unexpected format or content: %+v", configuration)
	}
	if configuration.FullPath != "config.json" {
		t.Fatalf("expect full path config.json, got %q", configuration.FullPath)
	}
}

// TestSearchFailureLogging verifies that consecutive failing searches keep a single failure state
// and that a successful search clears it, so that a broken server does not log at every refresh.
func TestSearchFailureLogging(t *testing.T) {
	stub := &stubConfigClient{searchErr: errors.New("server unavailable")}

	o := defaultOptions()
	o.client = stub
	o.groupName = "test-group"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	source := &Source{
		opts:     o,
		ctx:      ctx,
		versions: make(map[string]uint64),
		chListen: make(chan listenRequest),
		chCancel: make(chan string),
	}

	source.search()
	if !source.searchFailed {
		t.Fatal("expect the failure state to be recorded")
	}

	// A second failure keeps the state, so the warning is not repeated.
	source.search()
	if !source.searchFailed {
		t.Fatal("expect the failure state to survive consecutive failures")
	}

	stub.searchErr = nil
	source.search()
	if source.searchFailed {
		t.Fatal("expect the failure state to be cleared after a successful search")
	}
}

// TestWatcherNextSkipsEmptySignal verifies that a notification signal without pending
// configurations does not make Next return an empty result.
func TestWatcherNextSkipsEmptySignal(t *testing.T) {
	w := newWatcher(context.Background(), &Source{})

	// Simulate a leftover signal whose configurations were merged into a previous Next call.
	w.chWatch <- struct{}{}

	type result struct {
		configs []*config.Configuration
		err     error
	}

	done := make(chan result, 1)
	go func() {
		configs, err := w.Next()
		done <- result{configs: configs, err: err}
	}()

	select {
	case r := <-done:
		t.Fatalf("Next returned an empty result: %+v", r)
	case <-time.After(50 * time.Millisecond):
		// Next is still waiting for a real change, as expected.
	}

	w.notice(&config.Configuration{File: "a.json"})

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Next failed: %v", r.err)
		}
		if len(r.configs) != 1 || r.configs[0].File != "a.json" {
			t.Fatalf("unexpected configurations: %+v", r.configs)
		}
	case <-time.After(time.Second):
		t.Fatal("Next did not return the pending configuration")
	}
}

// TestSourceCloseStopsWatchers verifies that closing the source stops the registered watchers, so
// that Next returns instead of blocking forever on a watcher context the caller never cancels.
func TestSourceCloseStopsWatchers(t *testing.T) {
	source := NewSource(WithClient(&stubConfigClient{}))

	w, err := source.Watch(context.Background())
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}

	errs := make(chan error, 1)
	go func() {
		_, err := w.Next()
		errs <- err
	}()

	if err := source.Close(); err != nil {
		t.Fatalf("close source failed: %v", err)
	}

	select {
	case err := <-errs:
		if err == nil {
			t.Fatal("expect Next to return an error after the source is closed")
		}
	case <-time.After(time.Second):
		t.Fatal("Next blocked after the source was closed")
	}

	// A watcher registered after the shutdown must not be left blocking either.
	if _, err := source.Watch(context.Background()); err == nil {
		t.Fatal("expect Watch to fail on a closed source")
	}
}

// TestRefreshFixedFiles verifies that a fixed file list registers the listens once and never
// searches the group.
func TestRefreshFixedFiles(t *testing.T) {
	stub := &stubConfigClient{}

	source := NewSource(
		WithClient(stub),
		WithFiles("a.json", "b.yaml"),
	)

	deadline := time.Now().Add(2 * time.Second)
	for stub.listenCalls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := stub.listenCalls.Load(); got != 2 {
		t.Fatalf("expect 2 listen calls, got %d", got)
	}
	if got := stub.calls.Load(); got != 0 {
		t.Fatalf("expect no search calls, got %d", got)
	}

	if err := source.Close(); err != nil {
		t.Fatalf("close source failed: %v", err)
	}
}

// TestNormalizeFiles verifies that the fixed file list drops empty entries and duplicates.
func TestNormalizeFiles(t *testing.T) {
	tests := []struct {
		name string
		give []string
		want []string
	}{
		{name: "empty list", give: nil, want: nil},
		{name: "trim and dedupe", give: []string{" a.json ", "", "b.yaml", "a.json", "  "}, want: []string{"a.json", "b.yaml"}},
		{name: "all empty", give: []string{"", "  "}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeFiles(tt.give); !slices.Equal(got, tt.want) {
				t.Fatalf("expect %v, got %v", tt.want, got)
			}
		})
	}
}

// TestListClientExport verifies that Export calls the v3 admin export endpoint, parses the ZIP
// response and returns a map from dataId to content.
func TestListClientExport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos"+configExportPath {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(makeExportZip("test-group", map[string]string{
			"a.json": `{"key":"a"}`,
			"b.yaml": "key: b\n",
			"sub/c":  "ignored", // nested path falls back to base name
		})); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	o := defaultOptions()
	o.urls = []string{server.URL + "/nacos"}
	o.timeout = time.Second

	stub := &stubConfigClient{}
	client := newListConfigClient(stub, o)
	list, ok := client.(*listClient)
	if !ok {
		t.Fatalf("expect a list client, got %T", client)
	}

	content, err := list.Export("test-group")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if got := content["a.json"]; got != `{"key":"a"}` {
		t.Errorf("expect a.json content %q, got %q", `{"key":"a"}`, got)
	}
	if got := content["b.yaml"]; got != "key: b\n" {
		t.Errorf("expect b.yaml content %q, got %q", "key: b\n", got)
	}
	if got := content["c"]; got != "ignored" {
		t.Errorf("expect fallback base name c content %q, got %q", "ignored", got)
	}
}

// TestSourceLoadWithExport verifies that the first full load uses the export shortcut on a
// listClient and does not issue GetConfig calls per item.
func TestSourceLoadWithExport(t *testing.T) {
	stub := &stubConfigClient{}
	o := defaultOptions()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos" + configListPath:
			// SearchConfig returns dataId + md5 (no content, as usual for v3 admin list)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"totalCount":2,"pageNumber":1,"pagesAvailable":1,"pageItems":[{"dataId":"a.json","groupName":"test-group","md5":"m1"},{"dataId":"b.yaml","groupName":"test-group","md5":"m2"}]}}`))
		case "/nacos" + configExportPath:
			// Export returns ZIP with content
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(makeExportZip("test-group", map[string]string{
				"a.json": `content-of-a`,
				"b.yaml": "content-of-b",
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	o.urls = []string{server.URL + "/nacos"}
	o.groupName = "test-group"
	o.timeout = time.Second

	source := &Source{}
	source.opts = o
	source.versions = make(map[string]uint64)
	source.chListen = make(chan listenRequest)
	source.chCancel = make(chan string)
	source.cache = make(map[string]cachedItem)

	lc := newListConfigClient(stub, o)
	source.opts.client = lc

	configs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expect 2 configs, got %d", len(configs))
	}

	// Export shortcut: GetConfig should NOT have been called.
	if got := stub.getCalls.Load(); got != 0 {
		t.Fatalf("expect 0 get config calls with export shortcut, got %d", got)
	}

	// Content must match what the export returned.
	for _, c := range configs {
		switch c.File {
		case "a.json":
			if string(c.Content) != "content-of-a" {
				t.Errorf("a.json: expect %q, got %q", "content-of-a", string(c.Content))
			}
		case "b.yaml":
			if string(c.Content) != "content-of-b" {
				t.Errorf("b.yaml: expect %q, got %q", "content-of-b", string(c.Content))
			}
		default:
			t.Errorf("unexpected config file %s", c.File)
		}
	}
}

// makeExportZip builds a ZIP payload matching the v3 admin export format: each entry is named
// groupName/dataId and carries the configuration content.
func makeExportZip(group string, contents map[string]string) []byte {
	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)
	for name, content := range contents {
		f, _ := zw.Create(group + "/" + name)
		_, _ = f.Write([]byte(content))
	}
	_ = zw.Close()
	return buf.Bytes()
}

// TestPatternTerm verifies the literal segment pushed to the server as the blur keyword.
func TestPatternTerm(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{name: "prefix", pattern: "gate-*.json", want: "gate-"},
		{name: "path prefix", pattern: "rooms/*.yaml", want: "rooms/"},
		{name: "weak extension anchor", pattern: "*.json", want: ".json"},
		{name: "short first segment", pattern: "a*-gate.json", want: "-gate.json"},
		{name: "no literal", pattern: "*", want: ""},
		{name: "suffix", pattern: "*-gate.json", want: "-gate.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := patternTerm(tt.pattern); got != tt.want {
				t.Fatalf("patternTerm(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

// TestMatchDataId verifies that a mixed list of exact dataIds and "*" patterns selects the
// intended items and treats every other character literally.
func TestMatchDataId(t *testing.T) {
	files := []string{"config.json", "gate-*.json", "rooms/*.yaml", "weird[1]?.json"}

	tests := []struct {
		name   string
		dataId string
		want   bool
	}{
		{name: "exact", dataId: "config.json", want: true},
		{name: "prefix pattern", dataId: "gate-a.json", want: true},
		{name: "prefix pattern mismatch", dataId: "gate-a.yaml", want: false},
		{name: "star matches separator", dataId: "rooms/a/b.yaml", want: true},
		{name: "literal meta in exact entry", dataId: "weird[1]?.json", want: true},
		{name: "literal meta is not a wildcard", dataId: "weird1.json", want: false},
		{name: "unmatched", dataId: "other.json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchDataId(files, tt.dataId); got != tt.want {
				t.Fatalf("matchDataId(%q) = %v, want %v", tt.dataId, got, tt.want)
			}
		})
	}
}

// TestSourceLoadPatterns verifies that pattern mode searches the group, keeps only the matching
// items, adds the exact entries the search does not return and uses the server-side blur keyword
// only when it is enabled.
func TestSourceLoadPatterns(t *testing.T) {
	items := []model.ConfigItem{
		{DataId: "config.json", Md5: "m1"},
		{DataId: "gate-a.json", Md5: "m2"},
		{DataId: "gate-b.yaml", Md5: "m3"},
		{DataId: "other.json", Md5: "m4"},
	}
	want := []string{"config.json", "gate-a.json"}

	tests := []struct {
		name       string
		serverBlur bool
		wantSearch []string
	}{
		{name: "server blur", serverBlur: true, wantSearch: []string{"gate-"}},
		{name: "whole group scan", serverBlur: false, wantSearch: []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubConfigClient{}
			searched := make([]string, 0, 1)
			stub.searchFunc = func(param vo.SearchConfigParam) (*model.ConfigPage, error) {
				searched = append(searched, param.DataId)

				pageItems := make([]model.ConfigItem, 0, len(items))
				for _, item := range items {
					if param.DataId != "" && !strings.Contains(item.DataId, param.DataId) {
						continue
					}
					pageItems = append(pageItems, item)
				}

				return &model.ConfigPage{
					TotalCount:     len(pageItems),
					PageNumber:     1,
					PagesAvailable: 1,
					PageItems:      pageItems,
				}, nil
			}

			o := defaultOptions()
			o.client = stub
			o.groupName = "test-group"
			o.files = []string{"config.json", "gate-*.json"}
			o.serverBlur = tt.serverBlur

			source := &Source{opts: o, patternMode: true, cache: make(map[string]cachedItem)}

			configs, err := source.Load(context.Background())
			if err != nil {
				t.Fatalf("load configs failed: %v", err)
			}

			files := make([]string, 0, len(configs))
			for _, configuration := range configs {
				files = append(files, configuration.File)
			}
			if !slices.Equal(files, want) {
				t.Fatalf("expect %v, got %v", want, files)
			}
			if got := stub.getCalls.Load(); got != 2 {
				t.Fatalf("expect 2 get config calls, got %d", got)
			}
			if !slices.Equal(searched, tt.wantSearch) {
				t.Fatalf("expect searches %v, got %v", tt.wantSearch, searched)
			}
		})
	}
}

// TestSourceStorePatterns verifies that a pattern entry allows storing a matching new file and
// still rejects a file outside the list.
func TestSourceStorePatterns(t *testing.T) {
	stub := &stubConfigClient{}

	o := defaultOptions()
	o.client = stub
	o.mode = config.ReadWrite
	o.files = []string{"gate-*.json"}

	source := &Source{opts: o, patternMode: true}

	if err := source.Store(context.Background(), "gate-a.json", []byte("content")); err != nil {
		t.Fatalf("store matching file failed: %v", err)
	}

	if err := source.Store(context.Background(), "other.json", []byte("content")); err != errFileNotConfigured {
		t.Fatalf("expect errFileNotConfigured, got %v", err)
	}
}

// TestRefreshPatterns verifies that a pattern list searches the group instead of registering the
// configured entries once.
func TestRefreshPatterns(t *testing.T) {
	stub := &stubConfigClient{
		page: &model.ConfigPage{
			TotalCount:     1,
			PageNumber:     1,
			PagesAvailable: 1,
			PageItems:      []model.ConfigItem{{DataId: "gate-a.json"}},
		},
	}

	source := NewSource(WithClient(stub), WithFiles("gate-*.json"))

	deadline := time.Now().Add(2 * time.Second)
	for stub.listenCalls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := stub.listenCalls.Load(); got != 1 {
		t.Fatalf("expect 1 listen call, got %d", got)
	}
	if got := stub.calls.Load(); got == 0 {
		t.Fatal("expect the pattern list to search the group")
	}

	if err := source.Close(); err != nil {
		t.Fatalf("close source failed: %v", err)
	}
}
