package nacos

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/common/http_agent"
	"github.com/nacos-group/nacos-sdk-go/v2/common/nacos_error"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// configListPath is the Nacos v3 admin API that lists configurations. Unlike the v1 search API
// wrapped by the SDK, its page items contain the configuration metadata without the content of
// every configuration.
const configListPath = "/v3/admin/cs/config/list"

// configExportPath is the Nacos v3 admin API that exports all configurations of a group as a
// ZIP archive. Unlike the list endpoint, each entry carries the full configuration content.
const configExportPath = "/v3/admin/cs/config/export"

// v3LoginPath is the Nacos 3.x authentication endpoint. The SDK security proxy still targets the
// removed /v1/auth/users/login path, so the list client authenticates against this endpoint directly.
const v3LoginPath = "/v3/auth/user/login"

// reLoginWindowSeconds is the minimum interval between forced login attempts after a 403, matching
// the SDK throttle to avoid login storms.
const reLoginWindowSeconds = 60

// maxErrorBodyLength limits the response body embedded in an error message, so that an oversized
// error page is not carried around in memory and logs.
const maxErrorBodyLength = 512

// listClient decorates a config client so that SearchConfig lists configurations through the Nacos
// v3 admin API. Every other method is delegated to the wrapped client.
//
// The v3 admin API is not available on Nacos servers earlier than 3.0, and an account may lack the
// permission to use it. When a list request reports such a permanent failure, the client falls back
// to the wrapped SearchConfig and disables the v3 path for the rest of its lifetime.
type listClient struct {
	config_client.IConfigClient
	servers      []constant.ServerConfig // Server address list used by the list requests
	tenant       string                  // Namespace id
	username     string                  // Nacos account username for v3 admin auth
	password     string                  // Nacos account password for v3 admin auth
	timeout      uint64                  // Request timeout in milliseconds
	agent        http_agent.IHttpAgent   // HTTP agent shared by the list requests
	accessToken  atomic.Value            // Cached access token (string), set by login
	tokenTtl     int64                   // Token lifetime in seconds, 0 when unknown
	lastRefresh  int64                   // Unix timestamp of the last successful login
	lastAttempt  int64                   // Unix timestamp of the last login attempt, successful or not
	reLoginFlag  atomic.Bool             // Whether the next login must bypass the ttl window
	loginMu      sync.Mutex              // Serializes concurrent login attempts
	disabled     atomic.Bool             // Whether the v3 path is permanently unavailable
	warnOnce     sync.Once               // Ensures the unavailable warning is logged once
	fallbackOnce sync.Once               // Ensures the transient failure warning is logged once
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
// The caller owns the returned client and must call CloseClient when it is no longer needed. The
// wrapper starts no goroutine of its own: the access token is refreshed lazily when a list request
// is rejected.
func NewListConfigClient(client config_client.IConfigClient, opts ...Option) config_client.IConfigClient {
	if client == nil {
		return nil
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return newListConfigClient(client, o)
}

// newListConfigClient wraps client with the v3 list implementation. It returns client unchanged when
// the options describe a deployment that cannot use the v3 admin API.
func newListConfigClient(client config_client.IConfigClient, o *options) config_client.IConfigClient {
	if client == nil {
		return nil
	}

	// Avoid wrapping a client that already uses the v3 list implementation, which would issue the
	// same request twice on failure.
	if _, ok := client.(*listClient); ok {
		return client
	}

	if o == nil {
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

	// The access token is fetched lazily when the first list request is rejected, so the wrapper
	// does not need a periodic refresh goroutine. A login here is best-effort: if it fails the
	// first request will trigger it again.
	agent := &http_agent.HttpAgent{TlsConfig: clientConfig.TLSCfg}

	lc := &listClient{
		IConfigClient: client,
		servers:       serverConfigs,
		tenant:        o.namespaceId,
		username:      o.username,
		password:      o.password,
		timeout:       uint64(o.timeout.Milliseconds()),
		agent:         agent,
	}

	_ = lc.login()

	return lc
}

// SearchConfig lists configurations through the v3 admin API. It falls back to the wrapped client
// when the API is unsupported by the server or when the caller asks for an accurate search.
func (c *listClient) SearchConfig(param vo.SearchConfigParam) (*model.ConfigPage, error) {
	if c.disabled.Load() || (param.Search != "" && param.Search != "blur") {
		return c.IConfigClient.SearchConfig(param)
	}

	page, err := c.list(param)
	if err != nil && isAuthError(err) {
		if c.username == "" {
			// The v3 admin API requires authentication, but no credentials are configured. On
			// Nacos 3.x the SDK fallback also targets the v3 admin list and would fail with the
			// same 403, so falling back only retries the same failing request. Keep the v3 path
			// enabled and return a clear error so every subsequent call surfaces it rather than
			// silently falling through to the SDK's opaque retry error.
			return nil, errNoCredentials
		}

		// The cached access token may have expired; refresh it once and retry.
		c.invalidateToken()
		if loginErr := c.login(); loginErr != nil {
			log.Warnf("nacos v3 login failed after auth rejection: %v", loginErr)
		}
		page, err = c.list(param)
	}
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

// CloseClient closes the wrapped client. The list client holds no goroutine to stop.
func (c *listClient) CloseClient() {
	c.IConfigClient.CloseClient()
}

// login authenticates against the Nacos 3.x /v3/auth/user/login endpoint and caches the access
// token. It is idempotent and throttled: repeated calls within the token TTL window reuse the
// cached token, a forced re-login after a 403 is spaced by reLoginWindowSeconds, and a failed
// login backs off for the same window so that an unreachable or misconfigured auth endpoint is not
// probed on every request.
func (c *listClient) login() error {
	if c.username == "" {
		// No credentials configured; the server may still allow anonymous access.
		return nil
	}

	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	now := time.Now().Unix()

	switch {
	case c.reLoginFlag.Load():
		// The token was rejected, so the ttl window does not apply. A successful login within the
		// window still means the token is fresh, keeping the previous rate limit for retries.
		if c.lastRefresh > 0 && now-c.lastRefresh < reLoginWindowSeconds {
			return nil
		}
	case c.accessToken.Load() != nil && c.tokenTtl > 0:
		// Refresh two windows before expiration to keep the token valid.
		refreshAt := c.lastRefresh + c.tokenTtl - 2*(c.tokenTtl/10)
		if now < refreshAt {
			return nil
		}
	case c.lastAttempt > 0 && now-c.lastAttempt < reLoginWindowSeconds:
		// The previous attempt failed; wait for the backoff window before probing the endpoint
		// again so that every request does not pay for a full round of login timeouts.
		return nil
	}

	c.lastAttempt = now

	var lastErr error

	for _, server := range c.servers {
		token, ttl, err := c.loginAt(server)
		if err != nil {
			lastErr = err
			continue
		}

		c.accessToken.Store(token)
		c.tokenTtl = ttl
		c.lastRefresh = now
		c.reLoginFlag.Store(false)
		return nil
	}

	if lastErr == nil {
		lastErr = errors.New("nacos v3 login failed: no server available")
	}

	return lastErr
}

// serverAddress builds an absolute URL for the given server and API path. The context path is
// normalized to a single leading slash and no trailing slash.
func serverAddress(server constant.ServerConfig, apiPath string) string {
	contextPath := server.ContextPath
	if contextPath == "" {
		contextPath = constant.WEB_CONTEXT
	}
	contextPath = strings.Trim(contextPath, "/")

	scheme := server.Scheme
	if scheme == "" {
		scheme = "http"
	}

	address := scheme + "://" + server.IpAddr + ":" + strconv.FormatUint(server.Port, 10)
	if contextPath != "" {
		address += "/" + contextPath
	}

	return address + apiPath
}

// loginAt sends one login request to the given server and returns the access token and TTL.
func (c *listClient) loginAt(server constant.ServerConfig) (token string, ttl int64, err error) {
	reqURL := serverAddress(server, v3LoginPath)

	header := http.Header{"content-type": []string{"application/x-www-form-urlencoded"}}

	resp, err := c.agent.Post(reqURL, header, c.timeout, map[string]string{
		"username": c.username,
		"password": c.password,
	})
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, nacos_error.NewNacosError(strconv.Itoa(resp.StatusCode), truncateBody(body), nil)
	}

	var result struct {
		AccessToken string `json:"accessToken"`
		TokenTtl    int64  `json:"tokenTtl"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, err
	}

	if result.AccessToken == "" {
		return "", 0, errors.New("nacos v3 login response missing accessToken")
	}

	return result.AccessToken, result.TokenTtl, nil
}

// truncateBody truncates a response body for use in an error message.
func truncateBody(body []byte) string {
	if len(body) <= maxErrorBodyLength {
		return string(body)
	}

	return string(body[:maxErrorBodyLength])
}

// getAccessToken returns the cached access token, logging in if it is missing or expired. It
// returns an empty string when no credentials are configured.
func (c *listClient) getAccessToken() (string, error) {
	if v := c.accessToken.Load(); v != nil {
		return v.(string), nil
	}

	if err := c.login(); err != nil {
		return "", err
	}

	if v := c.accessToken.Load(); v != nil {
		return v.(string), nil
	}

	return "", nil
}

// invalidateToken marks the cached token as rejected so the next login bypasses the TTL window.
func (c *listClient) invalidateToken() {
	c.reLoginFlag.Store(true)
}

// Export exports the content of every configuration under group as a map keyed by dataId.
// The v3 admin export endpoint returns a ZIP archive whose entries are named as group/dataId.
// Export returns nil when the v3 admin API is permanently unavailable.
func (c *listClient) Export(group string) (map[string]string, error) {
	if c.disabled.Load() {
		return nil, nil
	}

	params := map[string]string{"groupName": group, "group": group}
	if c.tenant != "" {
		params["tenant"] = c.tenant
		params["namespaceId"] = c.tenant
	}

	raw, err := c.doGet(configExportPath, params)
	if err != nil && isAuthError(err) {
		if c.username == "" {
			// Same reasoning as SearchConfig: the SDK fallback for content fetch (GetConfig) may
			// also require auth on Nacos 3.x. Return a clear error rather than failing opaquely.
			return nil, errNoCredentials
		}

		c.invalidateToken()
		if loginErr := c.login(); loginErr != nil {
			log.Warnf("nacos v3 login failed after auth rejection: %v", loginErr)
		}
		raw, err = c.doGet(configExportPath, params)
	}
	if err != nil {
		if isListUnavailable(err) {
			c.disabled.Store(true)
			c.warnOnce.Do(func() {
				log.Warnf("nacos v3 config export is unavailable, falling back to per-config fetch: %v", err)
			})
		}
		return nil, err
	}

	return parseExport(raw, group)
}

// doGet sends one HTTP GET request to the given path on every configured server in turn and returns
// the body of the first successful response. The cached access token is injected once, so that a
// login attempt is not repeated for every server.
func (c *listClient) doGet(path string, params map[string]string) ([]byte, error) {
	if token, err := c.getAccessToken(); err == nil && token != "" {
		params[constant.KEY_ACCESS_TOKEN] = token
	}

	var lastErr error

	for _, server := range c.servers {
		body, err := c.get(server, path, params)
		if err != nil {
			lastErr = err
			continue
		}

		return body, nil
	}

	if lastErr == nil {
		lastErr = errors.New("nacos config request failed: no server available")
	}

	return nil, lastErr
}

// get sends one HTTP GET request to the given server and returns the response body.
func (c *listClient) get(server constant.ServerConfig, path string, params map[string]string) ([]byte, error) {
	resp, err := c.agent.Get(serverAddress(server, path), http.Header{}, c.timeout, params)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nacos_error.NewNacosError(strconv.Itoa(resp.StatusCode), truncateBody(body), nil)
	}

	return body, nil
}

// parseExport parses the ZIP archive returned by the v3 admin export endpoint and returns a map
// from dataId to content. ZIP entries are named as groupName/dataId so the group prefix is
// stripped.
func parseExport(data []byte, group string) (map[string]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(zr.File))
	prefix := group + "/"

	for _, f := range zr.File {
		// Skip directory entries and empty files, whose content cannot be distinguished from a
		// missing entry in the returned map.
		if info := f.FileInfo(); info.IsDir() || info.Size() == 0 {
			continue
		}

		// Entries are named groupName/dataId; strip the group prefix when present.
		name, _ := strings.CutPrefix(f.Name, prefix)
		// Strip any remaining directory components inside the group.
		name = path.Base(name)
		if name == "." || name == "/" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}

		result[name] = string(content)
	}

	return result, nil
}

// list requests one page from the v3 admin API, trying every configured server in turn.
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

	raw, err := c.doGet(configListPath, params)
	if err != nil {
		return nil, err
	}

	return parseConfigList(raw)
}

// parseConfigList decodes one list response.
func parseConfigList(raw []byte) (*model.ConfigPage, error) {
	var result configListResult
	if err := json.Unmarshal(raw, &result); err != nil {
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

// isAuthError reports whether the request was rejected because the access token is missing or
// expired, so that a login may fix it.
func isAuthError(err error) bool {
	var nacosErr *nacos_error.NacosError
	if errors.As(err, &nacosErr) {
		code := nacosErr.ErrorCode()
		return code == "401" || code == "403"
	}

	return false
}

// isListUnavailable reports whether the list request failed in a way that will not recover by
// retrying: the server does not provide the v3 admin API, or the configured account is not allowed
// to use it. Transient statuses such as 401, 408 and 429 keep the v3 path enabled, because the
// access token can be refreshed or the endpoint can recover.
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

	return false
}

// isUnavailableCode reports whether the HTTP status code means the v3 list endpoint cannot be used
// by this deployment. A 401 is not included: it may be caused by an expired token or a temporarily
// unavailable login endpoint, so the v3 path keeps retrying with the login backoff instead of being
// disabled for the rest of the process lifetime.
func isUnavailableCode(code string) bool {
	switch code {
	case "400", "403", "404", "405":
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

// errNoCredentials is returned when the v3 admin API rejects an anonymous request but no username
// or password is configured. It avoids the futile fallback to the SDK, which would hit the same
// v3 admin endpoint on Nacos 3.x and fail identically.
var errNoCredentials = errors.New("nacos admin API requires authentication: configure username and password (e.g. WithUsername/WithPassword) or disable nacos.core.auth.admin.enabled on the server")

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
