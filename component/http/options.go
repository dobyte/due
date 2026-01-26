package http

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/gofiber/fiber/v3"
)

const (
	defaultName            = "http"          // 默认HTTP服务名称
	defaultAddr            = ":8080"         // 默认监听地址
	defaultBodyLimit       = 4 * 1024 * 1024 // 默认body大小
	defaultConcurrency     = 256 * 1024      // 默认最大并发连接数
	defaultReadBufferSize  = 4096            // 默认读取缓冲区大小
	defaultWriteBufferSize = 4096            // 默认写入缓冲区大小
)

const (
	defaultNameKey                         = "etc.http.name"
	defaultAddrKey                         = "etc.http.addr"
	defaultConsoleKey                      = "etc.http.console"
	defaultBodyLimitKey                    = "etc.http.bodyLimit"
	defaultConcurrencyKey                  = "etc.http.concurrency"
	defaultStrictRoutingKey                = "etc.http.strictRouting"
	defaultCaseSensitiveKey                = "etc.http.caseSensitive"
	defaultDisableHeadAutoRegisterKey      = "etc.http.disableHeadAutoRegister"
	defaultImmutableKey                    = "etc.http.immutable"
	defaultUnescapePathKey                 = "etc.http.unescapePath"
	defaultViewsLayoutKey                  = "etc.http.viewsLayout"
	defaultPassLocalsToViewsKey            = "etc.http.passLocalsToViews"
	defaultReadBufferSizeKey               = "etc.http.readBufferSize"
	defaultWriteBufferSizeKey              = "etc.http.writeBufferSize"
	defaultProxyHeaderKey                  = "etc.http.proxyHeader"
	defaultDisableKeepaliveKey             = "etc.http.disableKeepalive"
	defaultDisableDefaultDateKey           = "etc.http.disableDefaultDate"
	defaultDisableDefaultContentTypeKey    = "etc.http.disableDefaultContentType"
	defaultDisableHeaderNormalizingKey     = "etc.http.disableHeaderNormalizing"
	defaultStreamRequestBodyKey            = "etc.http.streamRequestBody"
	defaultDisablePreParseMultipartFormKey = "etc.http.disablePreParseMultipartForm"

	defaultKeyFileKey  = "etc.http.keyFile"
	defaultCertFileKey = "etc.http.certFile"
	defaultCorsKey     = "etc.http.cors"
	defaultSwaggerKey  = "etc.http.swagger"
)

type Option func(o *options)

type options struct {
	name                         string             // HTTP服务名称
	addr                         string             // 监听地址
	console                      bool               // 是否启用控制台输出
	strictRouting                bool               // 是否启用严格路由模式，默认为false，启用后"/foo"与"/foo/"为两个不同的路由
	caseSensitive                bool               // 是否区分路由大小写，默认为false， 启用后"/FoO"与"/foo"为两个不同的路由
	disableHeadAutoRegister      bool               // 是否禁用HEAD方法自动注册，默认为false
	immutable                    bool               // 是否启用不可变路由，默认为false
	unescapePath                 bool               // 是否unescape路径参数，默认为false
	bodyLimit                    int                // body大小，默认为4 * 1024 * 1024
	concurrency                  int                // 最大并发连接数，默认为256 * 1024
	views                        fiber.Views        // 视图引擎
	viewsLayout                  string             // 视图布局
	passLocalsToViews            bool               // 是否将上下文 locals 传递给视图引擎
	readBufferSize               int                // 读取缓冲区大小，默认为4096
	writeBufferSize              int                // 写入缓冲区大小，默认为4096
	proxyHeader                  string             // 代理头部
	errorHandler                 fiber.ErrorHandler // 错误处理函数
	disableKeepalive             bool               // 是否禁用keepalive，默认为false
	disableDefaultDate           bool               // 是否禁用默认日期，默认为false
	disableDefaultContentType    bool               // 是否禁用默认Content-Type，默认为false
	disableHeaderNormalizing     bool               // 是否禁用默认头部归一化，默认为false
	streamRequestBody            bool               // 是否流式请求体，默认为false
	disablePreParseMultipartForm bool               // 是否禁用预解析multipart/form-data，默认为false

	certFile    string                // 证书文件
	keyFile     string                // 秘钥文件
	registry    registry.Registry     // 服务注册器
	transporter transport.Transporter // 消息传输器

	corsOpts    CorsOptions // 跨域配置
	swagOpts    SwagOptions // swagger配置
	middlewares []any       // 中间件
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
		name:                         etc.Get(defaultNameKey, defaultName).String(),
		addr:                         etc.Get(defaultAddrKey, defaultAddr).String(),
		console:                      etc.Get(defaultConsoleKey).Bool(),
		strictRouting:                etc.Get(defaultStrictRoutingKey).Bool(),
		caseSensitive:                etc.Get(defaultCaseSensitiveKey).Bool(),
		disableHeadAutoRegister:      etc.Get(defaultDisableHeadAutoRegisterKey).Bool(),
		immutable:                    etc.Get(defaultImmutableKey).Bool(),
		unescapePath:                 etc.Get(defaultUnescapePathKey).Bool(),
		bodyLimit:                    int(etc.Get(defaultBodyLimitKey, defaultBodyLimit).B()),
		concurrency:                  etc.Get(defaultConcurrencyKey, defaultConcurrency).Int(),
		viewsLayout:                  etc.Get(defaultViewsLayoutKey).String(),
		passLocalsToViews:            etc.Get(defaultPassLocalsToViewsKey).Bool(),
		readBufferSize:               etc.Get(defaultReadBufferSizeKey, defaultReadBufferSize).Int(),
		writeBufferSize:              etc.Get(defaultWriteBufferSizeKey, defaultWriteBufferSize).Int(),
		proxyHeader:                  etc.Get(defaultProxyHeaderKey).String(),
		disableKeepalive:             etc.Get(defaultDisableKeepaliveKey).Bool(),
		disableDefaultDate:           etc.Get(defaultDisableDefaultDateKey).Bool(),
		disableDefaultContentType:    etc.Get(defaultDisableDefaultContentTypeKey).Bool(),
		disableHeaderNormalizing:     etc.Get(defaultDisableHeaderNormalizingKey).Bool(),
		streamRequestBody:            etc.Get(defaultStreamRequestBodyKey).Bool(),
		disablePreParseMultipartForm: etc.Get(defaultDisablePreParseMultipartFormKey).Bool(),

		keyFile:  etc.Get(defaultKeyFileKey).String(),
		certFile: etc.Get(defaultCertFileKey).String(),
		corsOpts: CorsOptions{},
		swagOpts: SwagOptions{},
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

// WithStrictRouting 设置是否启用严格路由模式
func WithStrictRouting(enable bool) Option {
	return func(o *options) { o.strictRouting = enable }
}

// WithCaseSensitive 设置是否区分路由大小写
func WithCaseSensitive(enable bool) Option {
	return func(o *options) { o.caseSensitive = enable }
}

// WithDisableHeadAutoRegister 设置是否禁用HEAD自动注册
func WithDisableHeadAutoRegister(disable bool) Option {
	return func(o *options) { o.disableHeadAutoRegister = disable }
}

// WithImmutable 设置是否启用不可变路由
func WithImmutable(enable bool) Option {
	return func(o *options) { o.immutable = enable }
}

// WithUnescapePath 设置是否unescape路径参数
func WithUnescapePath(enable bool) Option {
	return func(o *options) { o.unescapePath = enable }
}

// WithBodyLimit 设置body大小
func WithBodyLimit(bodyLimit int) Option {
	return func(o *options) { o.bodyLimit = bodyLimit }
}

// WithConcurrency 设置最大并发连接数
func WithConcurrency(concurrency int) Option {
	return func(o *options) { o.concurrency = concurrency }
}

// WithViews 设置视图引擎
func WithViews(views fiber.Views) Option {
	return func(o *options) { o.views = views }
}

// WithViewsLayout 设置视图布局
func WithViewsLayout(layout string) Option {
	return func(o *options) { o.viewsLayout = layout }
}

// WithPassLocalsToViews 设置是否将上下文 locals 传递给视图引擎
func WithPassLocalsToViews(enable bool) Option {
	return func(o *options) { o.passLocalsToViews = enable }
}

// WithReadBufferSize 设置读取缓冲区大小
func WithReadBufferSize(size int) Option {
	return func(o *options) { o.readBufferSize = size }
}

// WithWriteBufferSize 设置写入缓冲区大小
func WithWriteBufferSize(size int) Option {
	return func(o *options) { o.writeBufferSize = size }
}

// WithProxyHeader 设置代理头部
func WithProxyHeader(proxyHeader string) Option {
	return func(o *options) { o.proxyHeader = proxyHeader }
}

// WithErrorHandler 设置错误处理函数
func WithErrorHandler(errorHandler fiber.ErrorHandler) Option {
	return func(o *options) { o.errorHandler = errorHandler }
}

// WithDisableKeepalive 设置是否禁用keepalive
func WithDisableKeepalive(disable bool) Option {
	return func(o *options) { o.disableKeepalive = disable }
}

// WithDisableDefaultDate 设置是否禁用默认日期
func WithDisableDefaultDate(disable bool) Option {
	return func(o *options) { o.disableDefaultDate = disable }
}

// WithDisableDefaultContentType 设置是否禁用默认Content-Type
func WithDisableDefaultContentType(disable bool) Option {
	return func(o *options) { o.disableDefaultContentType = disable }
}

// WithDisableHeaderNormalizing 设置是否禁用默认头部归一化
func WithDisableHeaderNormalizing(disable bool) Option {
	return func(o *options) { o.disableHeaderNormalizing = disable }
}

// WithStreamRequestBody 设置是否流式请求体
func WithStreamRequestBody(enable bool) Option {
	return func(o *options) { o.streamRequestBody = enable }
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
