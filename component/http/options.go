package http

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

const (
	defaultName        = "http"          // 默认HTTP服务名称
	defaultAddr        = ":8080"         // 默认监听地址
	defaultBodyLimit   = 4 * 1024 * 1024 // 默认body大小
	defaultConcurrency = 256 * 1024      // 默认最大并发连接数
)

const (
	defaultNameKey          = "etc.http.name"
	defaultAddrKey          = "etc.http.addr"
	defaultConsoleKey       = "etc.http.console"
	defaultBodyLimitKey     = "etc.http.bodyLimit"
	defaultConcurrencyKey   = "etc.http.concurrency"
	defaultStrictRoutingKey = "etc.http.strictRouting"
	defaultCaseSensitiveKey = "etc.http.caseSensitive"
	defaultKeyFileKey       = "etc.http.keyFile"
	defaultCertFileKey      = "etc.http.certFile"
	defaultCorsKey          = "etc.http.cors"
	defaultSwaggerKey       = "etc.http.swagger"
)

type Option func(o *options)

type options struct {
	name          string                // HTTP服务名称
	addr          string                // 监听地址
	console       bool                  // 是否启用控制台输出
	bodyLimit     int                   // body大小，默认为4 * 1024 * 1024
	concurrency   int                   // 最大并发连接数，默认为256 * 1024
	strictRouting bool                  // 是否启用严格路由模式，默认为false，启用后"/foo"与"/foo/"为两个不同的路由
	caseSensitive bool                  // 是否区分路由大小写，默认为false， 启用后"/FoO"与"/foo"为两个不同的路由
	certFile      string                // 证书文件
	keyFile       string                // 秘钥文件
	registry      registry.Registry     // 服务注册器
	transporter   transport.Transporter // 消息传输器
	corsOpts      CorsOptions           // 跨域配置
	swagOpts      SwagOptions           // swagger配置
	middlewares   []any                 // 中间件
}

type CorsOptions struct {
	Enable              bool     `json:"enable"`              // 是否启用
	AllowOrigins        []string `json:"allowOrigins"`        // 允许跨域的请求源。默认为[]，即为允许所有请求源
	AllowMethods        []string `json:"allowMethods"`        // 允许跨域的请求方法。默认为["GET", "POST", "HEAD", "PUT", "DELETE", "PATCH"]
	AllowHeaders        []string `json:"allowHeaders"`        // 允许跨域的请求头部。默认为[]，即为允许所有请求头部
	AllowCredentials    bool     `json:"allowCredentials"`    // 当允许所有源时，根据CORS规范不允许携带凭据。默认为false
	ExposeHeaders       []string `json:"exposeHeaders"`       // 允许暴露给客户端的头部。默认为[]，即为允许暴露所有头部
	MaxAge              int      `json:"maxAge"`              // 浏览器缓存预检请求结果的时间。默认为0
	AllowPrivateNetwork bool     `json:"allowPrivateNetwork"` // 是否允许来自私有网络的请求。设置为true时，响应头Access-Control-Allow-Private-Network会被设置为true。默认为false
}

type SwagOptions struct {
	Enable           bool   `json:"enable"`           // 是否启用
	Title            string `json:"title"`            // 文档标题
	FilePath         string `json:"filePath"`         // 文档路径
	BasePath         string `json:"basePath"`         // 访问路径
	SwaggerBundleUrl string `json:"swaggerBundleUrl"` // swagger-ui-bundle.js地址
	SwaggerPresetUrl string `json:"swaggerPresetUrl"` // swagger-ui-standalone-preset.js地址
	SwaggerStylesUrl string `json:"swaggerStylesUrl"` // swagger-ui.css地址
}

func defaultOptions() *options {
	opts := &options{
		name:          etc.Get(defaultNameKey, defaultName).String(),
		addr:          etc.Get(defaultAddrKey, defaultAddr).String(),
		console:       etc.Get(defaultConsoleKey).Bool(),
		bodyLimit:     int(etc.Get(defaultBodyLimitKey, defaultBodyLimit).B()),
		concurrency:   etc.Get(defaultConcurrencyKey, defaultConcurrency).Int(),
		strictRouting: etc.Get(defaultStrictRoutingKey).Bool(),
		caseSensitive: etc.Get(defaultCaseSensitiveKey).Bool(),
		keyFile:       etc.Get(defaultKeyFileKey).String(),
		certFile:      etc.Get(defaultCertFileKey).String(),
		corsOpts:      CorsOptions{},
		swagOpts:      SwagOptions{},
	}

	if err := etc.Get(defaultCorsKey).Scan(&opts.corsOpts); err != nil {
		opts.corsOpts = CorsOptions{}
	}

	if err := etc.Get(defaultSwaggerKey).Scan(&opts.swagOpts); err != nil {
		opts.swagOpts = SwagOptions{}
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

// WithConsole 设置是否启用控制台输出
func WithConsole(enable bool) Option {
	return func(o *options) { o.console = enable }
}

// WithBodyLimit 设置body大小
func WithBodyLimit(bodyLimit int) Option {
	return func(o *options) { o.bodyLimit = bodyLimit }
}

// WithConcurrency 设置最大并发连接数
func WithConcurrency(concurrency int) Option {
	return func(o *options) { o.concurrency = concurrency }
}

// WithStrictRouting 设置是否启用严格路由模式
func WithStrictRouting(enable bool) Option {
	return func(o *options) { o.strictRouting = enable }
}

// WithCaseSensitive 设置是否区分路由大小写
func WithCaseSensitive(enable bool) Option {
	return func(o *options) { o.caseSensitive = enable }
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

// WithCorsOptions 设置跨域配置
func WithCorsOptions(corsOpts CorsOptions) Option {
	return func(o *options) { o.corsOpts = corsOpts }
}

// WithSwagOptions 设置swagger配置
func WithSwagOptions(swagOpts SwagOptions) Option {
	return func(o *options) { o.swagOpts = swagOpts }
}

// WithMiddlewares 设置中间件
func WithMiddlewares(middlewares ...any) Option {
	return func(o *options) { o.middlewares = middlewares }
}
