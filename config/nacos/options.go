package nacos

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
)

const (
	defaultMode        = config.ReadOnly
	defaultUrl         = "http://127.0.0.1:8848/nacos"
	defaultClusterName = "DEFAULT"
	defaultGroupName   = "DEFAULT_GROUP"
	defaultTimeout     = "3s"
	defaultNamespaceId = ""
	defaultEndpoint    = ""
	defaultRegionId    = ""
	defaultAccessKey   = ""
	defaultSecretKey   = ""
	defaultOpenKMS     = false
	defaultCacheDir    = "./run/nacos/config/cache"
	defaultUsername    = ""
	defaultPassword    = ""
	defaultLogDir      = "./run/nacos/config/log"
	defaultLogLevel    = "info"
)

const (
	defaultModeKey        = "etc.config.nacos.mode"
	defaultUrlsKey        = "etc.config.nacos.urls"
	defaultClusterNameKey = "etc.config.nacos.clusterName"
	defaultGroupNameKey   = "etc.config.nacos.groupName"
	defaultTimeoutKey     = "etc.config.nacos.timeout"
	defaultNamespaceIdKey = "etc.config.nacos.namespaceId"
	defaultEndpointKey    = "etc.config.nacos.endpoint"
	defaultRegionIdKey    = "etc.config.nacos.regionId"
	defaultAccessKeyKey   = "etc.config.nacos.accessKey"
	defaultSecretKeyKey   = "etc.config.nacos.secretKey"
	defaultOpenKMSKey     = "etc.config.nacos.openKMS"
	defaultCacheDirKey    = "etc.config.nacos.cacheDir"
	defaultUsernameKey    = "etc.config.nacos.username"
	defaultPasswordKey    = "etc.config.nacos.password"
	defaultLogDirKey      = "etc.config.nacos.logDir"
	defaultLogLevelKey    = "etc.config.nacos.logLevel"
)

// Option is a config option.
type Option func(o *options)

// options are the config options.
type options struct {
	// Context, defaults to context.Background.
	ctx context.Context

	// Read-write mode; supports read-only, write-only and read-write modes,
	// defaulting to read-only.
	mode config.Mode

	// Server addresses in the form [scheme://]ip:port[/nacos], defaulting to
	// []string{"http://127.0.0.1:8848/nacos"}.
	urls []string

	// External client; when present it takes precedence, defaulting to nil.
	client config_client.IConfigClient

	// Cluster name, defaulting to DEFAULT.
	clusterName string

	// Group name, defaulting to DEFAULT_GROUP.
	groupName string

	// Timeout for requests to the Nacos server, defaulting to 3 seconds.
	timeout time.Duration

	// Namespace id of ACM, defaulting to empty.
	namespaceId string

	// Endpoint required when using ACM, see
	// https://help.aliyun.com/document_detail/130146.html. Defaults to empty.
	endpoint string

	// regionId of ACM&KMS, used for config center authentication, defaulting to empty.
	regionId string

	// AccessKey of ACM&KMS, used for config center authentication, defaulting to empty.
	accessKey string

	// SecretKey of ACM&KMS, used for config center authentication, defaulting to empty.
	secretKey string

	// Whether KMS is enabled, see https://help.aliyun.com/product/28933.html.
	// The DataId must start with "cipher-" for the encryption and decryption
	// logic to take effect. Defaults to disabled.
	openKMS bool

	// Directory caching service information, defaulting to ./run/nacos/config/cache.
	cacheDir string

	// Username for Nacos server API authentication, defaulting to empty.
	username string

	// Password for Nacos server API authentication, defaulting to empty.
	password string

	// Log storage path, defaulting to ./run/nacos/config/log.
	logDir string

	// Log output level, defaulting to info.
	logLevel string
}

// defaultOptions returns the default config options.
func defaultOptions() *options {
	return &options{
		ctx:         context.Background(),
		mode:        config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
		urls:        etc.Get(defaultUrlsKey, []string{defaultUrl}).Strings(),
		clusterName: etc.Get(defaultClusterNameKey, defaultClusterName).String(),
		groupName:   etc.Get(defaultGroupNameKey, defaultGroupName).String(),
		timeout:     etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		namespaceId: etc.Get(defaultNamespaceIdKey, defaultNamespaceId).String(),
		endpoint:    etc.Get(defaultEndpointKey, defaultEndpoint).String(),
		regionId:    etc.Get(defaultRegionIdKey, defaultRegionId).String(),
		accessKey:   etc.Get(defaultAccessKeyKey, defaultAccessKey).String(),
		secretKey:   etc.Get(defaultSecretKeyKey, defaultSecretKey).String(),
		openKMS:     etc.Get(defaultOpenKMSKey, defaultOpenKMS).Bool(),
		cacheDir:    etc.Get(defaultCacheDirKey, defaultCacheDir).String(),
		username:    etc.Get(defaultUsernameKey, defaultUsername).String(),
		password:    etc.Get(defaultPasswordKey, defaultPassword).String(),
		logDir:      etc.Get(defaultLogDirKey, defaultLogDir).String(),
		logLevel:    etc.Get(defaultLogLevelKey, defaultLogLevel).String(),
	}
}

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithUrls sets the server addresses.
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient sets the external client.
func WithClient(client config_client.IConfigClient) Option {
	return func(o *options) { o.client = client }
}

// WithClusterName sets the cluster name.
func WithClusterName(clusterName string) Option {
	return func(o *options) { o.clusterName = clusterName }
}

// WithGroupName sets the group name.
func WithGroupName(groupName string) Option {
	return func(o *options) { o.groupName = groupName }
}

// WithTimeout sets the timeout for requests to the Nacos server.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithNamespaceId sets the namespace id of ACM.
func WithNamespaceId(namespaceId string) Option {
	return func(o *options) { o.namespaceId = namespaceId }
}

// WithEndpoint sets the endpoint of ACM.
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithRegionId sets the regionId of ACM&KMS.
func WithRegionId(regionId string) Option {
	return func(o *options) { o.regionId = regionId }
}

// WithAccessKey sets the AccessKey of ACM&KMS.
func WithAccessKey(accessKey string) Option {
	return func(o *options) { o.accessKey = accessKey }
}

// WithSecretKey sets the SecretKey of ACM&KMS.
func WithSecretKey(secretKey string) Option {
	return func(o *options) { o.secretKey = secretKey }
}

// WithOpenKMS sets whether KMS is enabled.
func WithOpenKMS(openKMS bool) Option {
	return func(o *options) { o.openKMS = openKMS }
}

// WithCacheDir sets the cache directory for service information.
func WithCacheDir(cacheDir string) Option {
	return func(o *options) { o.cacheDir = cacheDir }
}

// WithUsername sets the username for Nacos server API authentication.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password for Nacos server API authentication.
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithLogDir sets the log storage path.
func WithLogDir(logDir string) Option {
	return func(o *options) { o.logDir = logDir }
}

// WithLogLevel sets the log output level.
func WithLogLevel(logLevel string) Option {
	return func(o *options) { o.logLevel = logLevel }
}
