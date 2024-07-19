package nacos

import (
	"context"
	"github.com/dobyte/due/v2/etc"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"time"
)

const (
	defaultUrl         = "http://127.0.0.1:8848/nacos"
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
	defaultClusterName = "DEFAULT"
	defaultGroupName   = "DEFAULT_GROUP"
	defaultTimeout     = "3s"
)

const (
	defaultUrlsKey        = "etc.registry.nacos.urls"
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
	defaultClusterNameKey = "etc.registry.nacos.clusterName"
	defaultGroupNameKey   = "etc.registry.nacos.groupName"
	defaultTimeoutKey     = "etc.registry.nacos.timeout"
)

type Option func(o *options)

type options struct {
	// 上下文
	// 默认context.Background
	ctx context.Context

	// 服务器地址 [scheme://]ip:port[/nacos]
	// 默认为[]string{http://127.0.0.1:8848/nacos}
	urls []string

	// 外部客户端
	// 外部客户端配置，存在外部客户端时，优先使用外部客户端，默认为nil
	client naming_client.INamingClient

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

	// 是否开启kms，默认不开启，kms可以参考文档 https://help.aliyun.com/product/28933.html
	// 同时DataId必须以"cipher-"作为前缀才会启动加解密逻辑
	// 默认不开启
	openKMS bool

	// 缓存service信息的目录，默认是当前运行目录
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

	// 集群名称
	clusterName string

	// 群组名称
	// 默认为DEFAULT_GROUP
	groupName string

	// 上下文超时时间
	// 默认为3秒
	timeout time.Duration
}

func defaultOptions() *options {
	return &options{
		ctx:         context.Background(),
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
	}
}
