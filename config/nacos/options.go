package nacos

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"time"
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

type Option func(o *options)

type options struct {
	// 上下文
	// 默认context.Background
	ctx context.Context

	// 读写模式
	// 支持read-only、write-only和read-write三种模式，默认为read-only模式
	mode config.Mode

	// 服务器地址 [scheme://]ip:port[/nacos]
	// 默认为[]string{http://127.0.0.1:8848/nacos}
	urls []string

	// 外部客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client config_client.IConfigClient

	// 集群名称
	// 默认为DEFAULT
	clusterName string

	// 群组名称
	// 默认为DEFAULT_GROUP
	groupName string

	// 请求Nacos服务端超时时间
	// 默认为3秒
	timeout time.Duration

	// ACM的命名空间Id
	// 默认为空
	namespaceId string

	// 当使用ACM时，需要该配置. https://help.aliyun.com/document_detail/130146.html
	// 默认为空
	endpoint string

	// ACM&KMS的regionId，用于配置中心的鉴权
	// 默认为空
	regionId string

	// ACM&KMS的AccessKey，用于配置中心的鉴权
	// 默认为空
	accessKey string

	// ACM&KMS的SecretKey，用于配置中心的鉴权
	// 默认为空
	secretKey string

	// 是否开启kms，kms可以参考文档 https://help.aliyun.com/product/28933.html
	// 同时DataId必须以"cipher-"作为前缀才会启动加解密逻辑
	// 默认不开启
	openKMS bool

	// 缓存service信息的目录
	// 默认为./run/nacos/naming/cache
	cacheDir string

	// Nacos服务端的API鉴权Username
	// 默认为空
	username string

	// Nacos服务端的API鉴权Password
	// 默认为空
	password string

	// 日志存储路径
	// 默认为./run/nacos/naming/log
	logDir string

	// 日志输出级别
	// 默认为info
	logLevel string
}

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

// WithContext 设置context
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithMode 设置读写模式
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithUrls 设置服务器地址
func WithUrls(urls ...string) Option {
	return func(o *options) { o.urls = urls }
}

// WithClient 设置外部客户端
func WithClient(client config_client.IConfigClient) Option {
	return func(o *options) { o.client = client }
}

// WithClusterName 设置集群名称
func WithClusterName(clusterName string) Option {
	return func(o *options) { o.clusterName = clusterName }
}

// WithGroupName 设置群组名称
func WithGroupName(groupName string) Option {
	return func(o *options) { o.groupName = groupName }
}

// WithTimeout 设置请求Nacos服务端超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithNamespaceId 设置ACM的命名空间Id
func WithNamespaceId(namespaceId string) Option {
	return func(o *options) { o.namespaceId = namespaceId }
}

// WithEndpoint 设置ACM的服务端点
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithRegionId 设置ACM&KMS的regionId
func WithRegionId(regionId string) Option {
	return func(o *options) { o.regionId = regionId }
}

// WithAccessKey 设置ACM&KMS的AccessKey
func WithAccessKey(accessKey string) Option {
	return func(o *options) { o.accessKey = accessKey }
}

// WithSecretKey 设置ACM&KMS的SecretKey
func WithSecretKey(secretKey string) Option {
	return func(o *options) { o.secretKey = secretKey }
}

// WithOpenKMS 设置是否是否开启KMS
func WithOpenKMS(openKMS bool) Option {
	return func(o *options) { o.openKMS = openKMS }
}

// WithCacheDir 设置service信息的缓存目录
func WithCacheDir(cacheDir string) Option {
	return func(o *options) { o.cacheDir = cacheDir }
}

// WithUsername 设置Nacos服务端的API鉴权Username
func WithUsername(username string) Option {
	return func(o *options) { o.username = username }
}

// WithPassword 设置Nacos服务端的API鉴权Password
func WithPassword(password string) Option {
	return func(o *options) { o.password = password }
}

// WithLogDir 设置日志存储路径
func WithLogDir(logDir string) Option {
	return func(o *options) { o.logDir = logDir }
}

// WithLogLevel 设置日志输出级别
func WithLogLevel(logLevel string) Option {
	return func(o *options) { o.logLevel = logLevel }
}
