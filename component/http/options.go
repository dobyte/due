package http

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

const (
	defaultName = "http"  // 默认HTTP服务名称
	defaultAddr = ":8080" // 监听地址
)

const (
	defaultNameKey     = "etc.http.name"
	defaultAddrKey     = "etc.http.addr"
	defaultKeyFileKey  = "etc.http.keyFile"
	defaultCertFileKey = "etc.http.certFile"
	defaultSwaggerKey  = "etc.http.swagger"
)

type Option func(o *options)

type options struct {
	name        string                // HTTP服务名称
	addr        string                // 监听地址
	certFile    string                // 证书文件
	keyFile     string                // 秘钥文件
	registry    registry.Registry     // 服务注册器
	transporter transport.Transporter // 消息传输器
	swagger     Swagger               // swagger配置
	middlewares []any                 // 中间件
}

type Swagger struct {
	Enable   bool   `json:"enable"`   // 是否启用
	Title    string `json:"title"`    // 文档标题
	FilePath string `json:"filePath"` // 文档路径
	BasePath string `json:"basePath"` // 访问路径
}

func defaultOptions() *options {
	opts := &options{
		name:     defaultName,
		addr:     defaultAddr,
		keyFile:  etc.Get(defaultKeyFileKey).String(),
		certFile: etc.Get(defaultCertFileKey).String(),
		swagger:  Swagger{},
	}

	if name := etc.Get(defaultNameKey).String(); name != "" {
		opts.name = name
	}

	if addr := etc.Get(defaultAddrKey).String(); addr != "" {
		opts.addr = addr
	}

	if err := etc.Get(defaultSwaggerKey).Scan(&opts.swagger); err != nil {
		opts.swagger = Swagger{}
	}

	return opts
}

// WithName 设置实例名称
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithAddr 设置监听地址
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithCredentials 设置证书和秘钥
func WithCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.keyFile, o.certFile = keyFile, certFile }
}

// WithRegistry 设置服务注册器
func WithRegistry(r registry.Registry) Option {
	return func(o *options) { o.registry = r }
}

// WithTransporter 设置消息传输器
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) { o.transporter = transporter }
}

// WithSwagger 设置Swagger配置
func WithSwagger(swagger Swagger) Option {
	return func(o *options) { o.swagger = swagger }
}

// WithMiddlewares 设置中间件
func WithMiddlewares(middlewares ...any) Option {
	return func(o *options) { o.middlewares = middlewares }
}
