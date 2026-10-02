package nacos

import (
	"time"

	"github.com/dobyte/due/v2/etc"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
)

const (
	defaultUrl         = "http://127.0.0.1:8848/nacos"
	defaultClusterName = "DEFAULT"
	defaultGroupName   = "DEFAULT_GROUP"
	defaultTimeout     = "10s"
	defaultHeartbeat   = "5s"
	defaultNamespaceId = ""
	defaultEndpoint    = ""
	defaultRegionId    = ""
	defaultAccessKey   = ""
	defaultSecretKey   = ""
	defaultOpenKMS     = false
	defaultCacheDir    = "./run/nacos/naming/cache"
	defaultUsername    = ""
	defaultPassword    = ""
	defaultLogDir      = "./run/nacos/naming/log"
	defaultLogLevel    = "info"
)

const (
	defaultUrlsKey        = "etc.registry.nacos.urls"
	defaultClusterNameKey = "etc.registry.nacos.clusterName"
	defaultGroupNameKey   = "etc.registry.nacos.groupName"
	defaultTimeoutKey     = "etc.registry.nacos.timeout"
	defaultHeartbeatKey   = "etc.registry.nacos.heartbeat"
	defaultNamespaceIdKey = "etc.registry.nacos.namespaceId"
	defaultEndpointKey    = "etc.registry.nacos.endpoint"
	defaultRegionIdKey    = "etc.registry.nacos.regionId"
	defaultAccessKeyKey   = "etc.registry.nacos.accessKey"
	defaultSecretKeyKey   = "etc.registry.nacos.secretKey"
	defaultOpenKMSKey     = "etc.registry.nacos.openKMS"
	defaultCacheDirKey    = "etc.registry.nacos.cacheDir"
	defaultUsernameKey    = "etc.registry.nacos.username"
	defaultPasswordKey    = "etc.registry.nacos.password"
	defaultLogDirKey      = "etc.registry.nacos.logDir"
	defaultLogLevelKey    = "etc.registry.nacos.logLevel"
)

// Option is a service registry and discovery option.
type Option func(o *options)

// options holds the Nacos registry configuration.
type options struct {
	// Server addresses in the form [scheme://]ip:port[/nacos].
	// Defaults to []string{http://127.0.0.1:8848/nacos}.
	urls []string

	// External client.
	// When an external client is provided it takes precedence over the built-in client and defaults to nil.
	client naming_client.INamingClient

	// Cluster name.
	// Defaults to DEFAULT.
	clusterName string

	// Group name.
	// Defaults to DEFAULT_GROUP.
	groupName string

	// Timeout of requests to the Nacos server.
	// Defaults to 10 seconds.
	timeout time.Duration

	// Heartbeat interval of the Nacos client.
	// Defaults to 5 seconds.
	heartbeat time.Duration

	// Namespace ID of ACM.
	// Defaults to empty.
	namespaceId string

	// Endpoint required when ACM is used. See https://help.aliyun.com/document_detail/130146.html.
	// Defaults to empty.
	endpoint string

	// regionId of ACM and KMS, used for authentication of the configuration center.
	// Defaults to empty.
	regionId string

	// AccessKey of ACM and KMS, used for authentication of the configuration center.
	// Defaults to empty.
	accessKey string

	// SecretKey of ACM and KMS, used for authentication of the configuration center.
	// Defaults to empty.
	secretKey string

	// Whether KMS is enabled. See https://help.aliyun.com/product/28933.html.
	// The DataId must be prefixed with "cipher-" for the encryption and decryption logic to start.
	// Disabled by default.
	openKMS bool

	// Directory caching the service information.
	// Defaults to ./run/nacos/naming/cache.
	cacheDir string

	// Username for the Nacos server API authentication.
	// Defaults to empty.
	username string

	// Password for the Nacos server API authentication.
	// Defaults to empty.
	password string

	// Log storage path.
	// Defaults to ./run/nacos/naming/log.
	logDir string

	// Log output level.
	// Defaults to info.
	logLevel string
}

func defaultOptions() *options {
	return &options{
		urls:        etc.Get(defaultUrlsKey, []string{defaultUrl}).Strings(),
		clusterName: etc.Get(defaultClusterNameKey, defaultClusterName).String(),
		groupName:   etc.Get(defaultGroupNameKey, defaultGroupName).String(),
		timeout:     etc.Get(defaultTimeoutKey, defaultTimeout).Duration(),
		heartbeat:   etc.Get(defaultHeartbeatKey, defaultHeartbeat).Duration(),
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

// WithUrls sets the server addresses.
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient sets the external client.
func WithClient(client naming_client.INamingClient) Option {
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

// WithTimeout sets the timeout of requests to the Nacos server.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithHeartbeat sets the heartbeat interval.
func WithHeartbeat(heartbeat time.Duration) Option {
	return func(o *options) { o.heartbeat = heartbeat }
}

// WithNamespaceId sets the ACM namespace ID.
func WithNamespaceId(namespaceId string) Option {
	return func(o *options) { o.namespaceId = namespaceId }
}

// WithEndpoint sets the ACM server endpoint.
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithRegionId sets the regionId of ACM and KMS.
func WithRegionId(regionId string) Option {
	return func(o *options) { o.regionId = regionId }
}

// WithAccessKey sets the AccessKey of ACM and KMS.
func WithAccessKey(accessKey string) Option {
	return func(o *options) { o.accessKey = accessKey }
}

// WithSecretKey sets the SecretKey of ACM and KMS.
func WithSecretKey(secretKey string) Option {
	return func(o *options) { o.secretKey = secretKey }
}

// WithOpenKMS sets whether KMS is enabled.
func WithOpenKMS(openKMS bool) Option {
	return func(o *options) { o.openKMS = openKMS }
}

// WithCacheDir sets the directory caching the service information.
func WithCacheDir(cacheDir string) Option {
	return func(o *options) { o.cacheDir = cacheDir }
}

// WithUsername sets the username for the Nacos server API authentication.
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword sets the password for the Nacos server API authentication.
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
