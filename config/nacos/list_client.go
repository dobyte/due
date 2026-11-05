package nacos

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/http_agent"
	"github.com/nacos-group/nacos-sdk-go/v2/common/nacos_error"
	"github.com/nacos-group/nacos-sdk-go/v2/common/nacos_server"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// configListPath is the Nacos v3 admin API that lists configurations. Unlike the v1 search API
// wrapped by the SDK, its page items contain the configuration metadata without the content of
// every configuration.
const configListPath = "/v3/admin/cs/config/list"

// listClient decorates a config client so that SearchConfig lists configurations through the Nacos
// v3 admin API. Every other method is delegated to the wrapped client.
//
// The v3 admin API is not available on Nacos servers earlier than 3.0, and an account may lack the
// permission to use it. When a list request reports such a permanent failure, the client falls back
// to the wrapped SearchConfig and disables the v3 path for the rest of its lifetime.
type listClient struct {
	config_client.IConfigClient
	server       *nacos_server.NacosServer
	tenant       string
	timeout      uint64
	cancel       context.CancelFunc
	disabled     atomic.Bool
	warnOnce     sync.Once
	fallbackOnce sync.Once
}

// NewListConfigClient wraps client so that SearchConfig lists configurations through the Nacos v3
// admin API, which returns configuration metadata without the content of each configuration. All
// other methods are delegated to client.
//
// The options provide the server list, namespace and credentials used by the list request; omitted
// options fall back to the same etc keys as NewSource. The wrapped client is used as a fallback
// when the server does not provide the v3 admin API, and is returned unchanged for ACM endpoint or
// AccessKey deployments, where the admin API is not applicable. A nil client returns nil.
//
// The caller owns the returned client and must call CloseClient when it is no longer needed. Use
// WithContext to control the lifetime of the internal token refresh. When a username is configured,
// the wrapper keeps its own token refresh goroutine in addition to the one of the wrapped client.
func NewListConfigClient(client config_client.IConfigClient, opts ...Option) config_client.IConfigClient {
	if client == nil {
		return nil
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return newListConfigClient(client, o, o.ctx)
}

// newListConfigClient wraps client with the v3 list implementation. It returns client unchanged when
// the options describe a deployment that cannot use the v3 admin API.
func newListConfigClient(client config_client.IConfigClient, o *options, ctx context.Context) config_client.IConfigClient {
	if client == nil {
		return nil
	}

	// Avoid wrapping a client that already uses the v3 list implementation, which would issue the
	// same request twice on failure.
	if _, ok := client.(*listClient); ok {
		return client
	}

	// The v3 admin API requires a self-hosted Nacos server with a static address list. ACM endpoints
	// and AccessKey deployments keep using the wrapped client.
	if o.endpoint != "" || o.accessKey != "" {
		return client
	}

	serverConfigs, _, _ := newServerConfigs(o.urls)
	if len(serverConfigs) == 0 {
		return client
	}

	clientConfig := newClientConfig(o)
	listCtx, cancel := context.WithCancel(ctx)
	server, err := nacos_server.NewNacosServer(
		listCtx,
		serverConfigs,
		clientConfig,
		&http_agent.HttpAgent{TlsConfig: clientConfig.TLSCfg},
		uint64(o.timeout.Milliseconds()),
		"",
		nil,
	)
	if err != nil {
		cancel()
		log.Warnf("create nacos config list client failed: %v", err)
		return client
	}

	return &listClient{
		IConfigClient: client,
		server:        server,
		tenant:        o.namespaceId,
		timeout:       uint64(o.timeout.Milliseconds()),
		cancel:        cancel,
	}
}

// SearchConfig lists configurations through the v3 admin API. It falls back to the wrapped client
// when the API is unsupported by the server or when the caller asks for an accurate search.
func (c *listClient) SearchConfig(param vo.SearchConfigParam) (*model.ConfigPage, error) {
	if c.disabled.Load() || (param.Search != "" && param.Search != "blur") {
		return c.IConfigClient.SearchConfig(param)
	}

	page, err := c.list(param)
	if err == nil {
		return page, nil
	}

	if isListUnavailable(err) {
		c.disabled.Store(true)
		c.warnOnce.Do(func() {
			log.Warnf("nacos v3 config list is unavailable, falling back to the config client search: %v", err)
		})
	} else {
		c.fallbackOnce.Do(func() {
			log.Warnf("nacos v3 config list failed, falling back to the config client search: %v", err)
		})
	}

	return c.IConfigClient.SearchConfig(param)
}

// CloseClient stops the token refresh of the v3 list client and closes the wrapped client.
func (c *listClient) CloseClient() {
	c.cancel()
	c.IConfigClient.CloseClient()
}

// list requests one page from the v3 admin API.
func (c *listClient) list(param vo.SearchConfigParam) (*model.ConfigPage, error) {
	pageNo, pageSize := param.PageNo, param.PageSize
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	params := map[string]string{
		"search":    "blur",
		"dataId":    param.DataId,
		"group":     param.Group,
		"groupName": param.Group,
		"pageNo":    strconv.Itoa(pageNo),
		"pageSize":  strconv.Itoa(pageSize),
	}
	if c.tenant != "" {
		params["tenant"] = c.tenant
		params["namespaceId"] = c.tenant
	}
	if param.Tag != "" {
		params["tag"] = param.Tag
	}
	if param.AppName != "" {
		params["appName"] = param.AppName
	}

	raw, err := c.server.ReqConfigApi(configListPath, params, nil, http.MethodGet, c.timeout)
	if err != nil {
		return nil, err
	}

	var result configListResult
	if err = json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, &listError{code: result.Code, message: result.Message}
	}

	page := &model.ConfigPage{
		TotalCount:     result.Data.TotalCount,
		PageNumber:     result.Data.PageNumber,
		PagesAvailable: result.Data.PagesAvailable,
		PageItems:      make([]model.ConfigItem, 0, len(result.Data.PageItems)),
	}

	for _, item := range result.Data.PageItems {
		page.PageItems = append(page.PageItems, model.ConfigItem{
			DataId: item.DataId,
			Group:  item.GroupName,
			Tenant: item.NamespaceId,
			Md5:    item.Md5,
		})
	}

	return page, nil
}

// isListUnavailable reports whether the list request failed in a way that will not recover by
// retrying: the server does not provide the v3 admin API, or the configured account is not allowed
// to use it. Transient statuses such as 408 and 429 keep the v3 path enabled.
func isListUnavailable(err error) bool {
	var (
		nacosErr *nacos_error.NacosError
		listErr  *listError
	)

	if errors.As(err, &nacosErr) {
		return isUnavailableCode(nacosErr.ErrorCode())
	}

	if errors.As(err, &listErr) {
		return isUnavailableResultCode(listErr.code)
	}

	message := err.Error()

	return strings.Contains(message, "error code 400") ||
		strings.Contains(message, "error code 401") ||
		strings.Contains(message, "error code 403") ||
		strings.Contains(message, "error code 404") ||
		strings.Contains(message, "error code 405")
}

// isUnavailableCode reports whether the HTTP status code means the v3 list endpoint cannot be used
// by this deployment.
func isUnavailableCode(code string) bool {
	switch code {
	case "400", "401", "403", "404", "405":
		return true
	default:
		return false
	}
}

// isUnavailableResultCode reports whether a v3 result code means the list endpoint cannot be used
// by this deployment. The values match the Nacos ErrorCode constants for access denied, parameter
// validate error, API deprecated and API function disabled. Server and quota errors keep the v3 path
// enabled because they may recover.
func isUnavailableResultCode(code int) bool {
	switch code {
	case 10001, 20002, 40000, 40001:
		return true
	default:
		return false
	}
}

// listError is a v3 list failure carrying the Nacos result code, so that permanent failures can be
// distinguished from transient ones.
type listError struct {
	code    int
	message string
}

// Error returns the error message reported by the Nacos server.
func (e *listError) Error() string {
	if e.message != "" {
		return e.message
	}

	return "nacos config list request failed"
}

// configListResult is the v3 admin list response envelope.
type configListResult struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    configListPage `json:"data"`
}

// configListPage is the v3 admin list page payload. The items carry metadata only, so the content of
// every configuration is not transferred.
type configListPage struct {
	TotalCount     int              `json:"totalCount"`
	PageNumber     int              `json:"pageNumber"`
	PagesAvailable int              `json:"pagesAvailable"`
	PageItems      []configListItem `json:"pageItems"`
}

// configListItem is one configuration metadata item of a v3 admin list page.
type configListItem struct {
	DataId      string `json:"dataId"`
	GroupName   string `json:"groupName"`
	NamespaceId string `json:"namespaceId"`
	Md5         string `json:"md5"`
}
