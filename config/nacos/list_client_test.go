package nacos

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var _ config_client.IConfigClient = (*stubConfigClient)(nil)

// stubConfigClient is a config client whose SearchConfig reports how often the fallback is used.
type stubConfigClient struct {
	page     *model.ConfigPage
	calls    atomic.Int32
	getCalls atomic.Int32
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
	return nil
}

func (c *stubConfigClient) CancelListenConfig(vo.ConfigParam) error {
	return nil
}

func (c *stubConfigClient) SearchConfig(vo.SearchConfigParam) (*model.ConfigPage, error) {
	c.calls.Add(1)

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
