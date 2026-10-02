package http

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/gofiber/fiber/v3"
)

const (
	defaultName            = "http"          // Default HTTP service name
	defaultAddr            = ":8080"         // Default listen address
	defaultBodyLimit       = 4 * 1024 * 1024 // Default body size
	defaultConcurrency     = 256 * 1024      // Default maximum number of concurrent connections
	defaultReadBufferSize  = 4096            // Default read buffer size
	defaultWriteBufferSize = 4096            // Default write buffer size
)

const (
	defaultNameKey                         = "etc.http.name"
	defaultAddrKey                         = "etc.http.addr"
	defaultKeyFileKey                      = "etc.http.keyFile"
	defaultCertFileKey                     = "etc.http.certFile"
	defaultConsoleKey                      = "etc.http.console"
	defaultCorsKey                         = "etc.http.cors"
	defaultSwaggerKey                      = "etc.http.swagger"
	defaultProxyKey                        = "etc.http.proxy"
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
	defaultDisableKeepaliveKey             = "etc.http.disableKeepalive"
	defaultDisableDefaultDateKey           = "etc.http.disableDefaultDate"
	defaultDisableDefaultContentTypeKey    = "etc.http.disableDefaultContentType"
	defaultDisableHeaderNormalizingKey     = "etc.http.disableHeaderNormalizing"
	defaultStreamRequestBodyKey            = "etc.http.streamRequestBody"
	defaultDisablePreParseMultipartFormKey = "etc.http.disablePreParseMultipartForm"
	defaultReduceMemoryUsageKey            = "etc.http.reduceMemoryUsage"
	defaultEnableIPValidationKey           = "etc.http.enableIPValidation"
	defaultEnableSplittingOnParsersKey     = "etc.http.enableSplittingOnParsers"
)

// Option is an HTTP server configuration function.
type Option func(o *options)

type options struct {
	name                         string                // HTTP service name
	addr                         string                // Listen address
	certFile                     string                // Certificate file
	keyFile                      string                // Key file
	console                      bool                  // Whether console output is enabled
	corsOpts                     CorsOptions           // CORS configuration
	swagOpts                     SwagOptions           // Swagger configuration
	proxyOpts                    ProxyOptions          // Proxy configuration
	middlewares                  []any                 // Middlewares
	registry                     registry.Registry     // Service registry
	transporter                  transport.Transporter // Message transporter
	strictRouting                bool                  // Whether strict routing is enabled; defaults to false. When enabled, "/foo" and "/foo/" are two different routes
	caseSensitive                bool                  // Whether route matching is case sensitive; defaults to false. When enabled, "/FoO" and "/foo" are two different routes
	disableHeadAutoRegister      bool                  // Whether automatic HEAD method registration is disabled; defaults to false
	immutable                    bool                  // Whether immutable routing is enabled; defaults to false
	unescapePath                 bool                  // Whether path parameters are unescaped; defaults to false
	bodyLimit                    int                   // Body size; defaults to 4 * 1024 * 1024
	concurrency                  int                   // Maximum number of concurrent connections; defaults to 256 * 1024
	views                        fiber.Views           // View engine
	viewsLayout                  string                // View layout
	passLocalsToViews            bool                  // Whether context locals are passed to the view engine
	readBufferSize               int                   // Read buffer size; defaults to 4096
	writeBufferSize              int                   // Write buffer size; defaults to 4096
	errorHandler                 fiber.ErrorHandler    // Error handler
	disableKeepalive             bool                  // Whether keepalive is disabled; defaults to false
	disableDefaultDate           bool                  // Whether the default date header is disabled; defaults to false
	disableDefaultContentType    bool                  // Whether the default Content-Type is disabled; defaults to false
	disableHeaderNormalizing     bool                  // Whether default header normalization is disabled; defaults to false
	streamRequestBody            bool                  // Whether the request body is streamed; defaults to false
	disablePreParseMultipartForm bool                  // Whether pre-parsing of multipart/form-data is disabled; defaults to false
	reduceMemoryUsage            bool                  // Whether memory usage is reduced; defaults to false
	enableIPValidation           bool                  // Whether IP validation is enabled; defaults to false
	enableSplittingOnParsers     bool                  // Whether splitting the request body on parsers is enabled; defaults to false
}

type ProxyOptions struct {
	ProxyHeader string            `json:"proxyHeader"` // Proxy header; defaults to X-Forwarded-For
	TrustProxy  TrustProxyOptions `json:"trustProxy"`  // Trusted proxy configuration
}

type CorsOptions struct {
	Enable              bool     `json:"enable"`              // Whether CORS is enabled
	AllowOrigins        []string `json:"allowOrigins"`        // Allowed request origins. Defaults to [], which allows every origin
	AllowMethods        []string `json:"allowMethods"`        // Allowed request methods. Defaults to ["GET", "POST", "HEAD", "PUT", "DELETE", "PATCH"]
	AllowHeaders        []string `json:"allowHeaders"`        // Allowed request headers. Defaults to [], which allows every header
	AllowCredentials    bool     `json:"allowCredentials"`    // When every origin is allowed, credentials are not permitted by the CORS specification. Defaults to false
	ExposeHeaders       []string `json:"exposeHeaders"`       // Headers exposed to the client. Defaults to [], which exposes every header
	MaxAge              int      `json:"maxAge"`              // Time for which the browser caches the preflight request result. Defaults to 0
	AllowPrivateNetwork bool     `json:"allowPrivateNetwork"` // Whether requests from private networks are allowed. When true, the Access-Control-Allow-Private-Network response header is set to true. Defaults to false
}

type SwagOptions struct {
	Enable           bool   `json:"enable"`           // Whether Swagger is enabled
	Title            string `json:"title"`            // Document title
	FilePath         string `json:"filePath"`         // Document path
	BasePath         string `json:"basePath"`         // Access path
	SwaggerBundleUrl string `json:"swaggerBundleUrl"` // swagger-ui-bundle.js URL
	SwaggerPresetUrl string `json:"swaggerPresetUrl"` // swagger-ui-standalone-preset.js URL
	SwaggerStylesUrl string `json:"swaggerStylesUrl"` // swagger-ui.css URL
}

type TrustProxyOptions struct {
	Enable    bool     `json:"enable"`    // Whether it is enabled; defaults to false
	Proxies   []string `json:"proxies"`   // Proxies is the list of trusted proxy IP addresses or CIDR ranges
	LinkLocal bool     `json:"linkLocal"` // Whether all link-local IP ranges are trusted (for example 169.254.0.0/16, fe80::/10)
	Loopback  bool     `json:"loopback"`  // Whether all loopback IP ranges are trusted (for example 127.0.0.0/8, ::1/128)
	Private   bool     `json:"private"`   // Whether all private IP ranges are trusted (for example 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
}

// defaultOptions creates the default configuration.
//
// It reads each parameter from the configuration environment and fills in the default values.
func defaultOptions() *options {
	opts := &options{
		name:                         etc.Get(defaultNameKey, defaultName).String(),
		addr:                         etc.Get(defaultAddrKey, defaultAddr).String(),
		console:                      etc.Get(defaultConsoleKey).Bool(),
		keyFile:                      etc.Get(defaultKeyFileKey).String(),
		certFile:                     etc.Get(defaultCertFileKey).String(),
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
		disableKeepalive:             etc.Get(defaultDisableKeepaliveKey).Bool(),
		disableDefaultDate:           etc.Get(defaultDisableDefaultDateKey).Bool(),
		disableDefaultContentType:    etc.Get(defaultDisableDefaultContentTypeKey).Bool(),
		disableHeaderNormalizing:     etc.Get(defaultDisableHeaderNormalizingKey).Bool(),
		streamRequestBody:            etc.Get(defaultStreamRequestBodyKey).Bool(),
		disablePreParseMultipartForm: etc.Get(defaultDisablePreParseMultipartFormKey).Bool(),
		reduceMemoryUsage:            etc.Get(defaultReduceMemoryUsageKey).Bool(),
		enableIPValidation:           etc.Get(defaultEnableIPValidationKey).Bool(),
		enableSplittingOnParsers:     etc.Get(defaultEnableSplittingOnParsersKey).Bool(),
	}

	if err := etc.Get(defaultCorsKey).Scan(&opts.corsOpts); err != nil {
		log.Warnf("scan cors options failed: %v", err)
	}

	if err := etc.Get(defaultSwaggerKey).Scan(&opts.swagOpts); err != nil {
		log.Warnf("scan swag options failed: %v", err)
	}

	if err := etc.Get(defaultProxyKey).Scan(&opts.proxyOpts); err != nil {
		log.Warnf("scan proxy options failed: %v", err)
	}

	return opts
}

// WithName sets the instance name.
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithAddr sets the listen address.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithCredentials sets the certificate and key files.
func WithCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.keyFile, o.certFile = keyFile, certFile }
}

// WithConsole sets whether console output is enabled.
func WithConsole(enable bool) Option {
	return func(o *options) { o.console = enable }
}

// WithRegistry sets the service registry.
func WithRegistry(r registry.Registry) Option {
	return func(o *options) { o.registry = r }
}

// WithTransporter sets the message transporter.
func WithTransporter(transporter transport.Transporter) Option {
	return func(o *options) { o.transporter = transporter }
}

// WithCorsOptions sets the CORS configuration.
func WithCorsOptions(corsOpts CorsOptions) Option {
	return func(o *options) { o.corsOpts = corsOpts }
}

// WithSwagOptions sets the Swagger configuration.
func WithSwagOptions(swagOpts SwagOptions) Option {
	return func(o *options) { o.swagOpts = swagOpts }
}

// WithProxyOptions sets the proxy configuration.
func WithProxyOptions(proxyOpts ProxyOptions) Option {
	return func(o *options) { o.proxyOpts = proxyOpts }
}

// WithMiddlewares sets the middlewares.
func WithMiddlewares(middlewares ...any) Option {
	return func(o *options) { o.middlewares = middlewares }
}

// WithStrictRouting sets whether strict routing is enabled.
func WithStrictRouting(enable bool) Option {
	return func(o *options) { o.strictRouting = enable }
}

// WithCaseSensitive sets whether route matching is case sensitive.
func WithCaseSensitive(enable bool) Option {
	return func(o *options) { o.caseSensitive = enable }
}

// WithDisableHeadAutoRegister sets whether automatic HEAD registration is disabled.
func WithDisableHeadAutoRegister(disable bool) Option {
	return func(o *options) { o.disableHeadAutoRegister = disable }
}

// WithImmutable sets whether immutable routing is enabled.
func WithImmutable(enable bool) Option {
	return func(o *options) { o.immutable = enable }
}

// WithUnescapePath sets whether path parameters are unescaped.
func WithUnescapePath(enable bool) Option {
	return func(o *options) { o.unescapePath = enable }
}

// WithBodyLimit sets the body size limit.
func WithBodyLimit(bodyLimit int) Option {
	return func(o *options) { o.bodyLimit = bodyLimit }
}

// WithConcurrency sets the maximum number of concurrent connections.
func WithConcurrency(concurrency int) Option {
	return func(o *options) { o.concurrency = concurrency }
}

// WithViews sets the view engine.
func WithViews(views fiber.Views) Option {
	return func(o *options) { o.views = views }
}

// WithViewsLayout sets the view layout.
func WithViewsLayout(layout string) Option {
	return func(o *options) { o.viewsLayout = layout }
}

// WithPassLocalsToViews sets whether context locals are passed to the view engine.
func WithPassLocalsToViews(enable bool) Option {
	return func(o *options) { o.passLocalsToViews = enable }
}

// WithReadBufferSize sets the read buffer size.
func WithReadBufferSize(size int) Option {
	return func(o *options) { o.readBufferSize = size }
}

// WithWriteBufferSize sets the write buffer size.
func WithWriteBufferSize(size int) Option {
	return func(o *options) { o.writeBufferSize = size }
}

// WithErrorHandler sets the error handler.
func WithErrorHandler(errorHandler fiber.ErrorHandler) Option {
	return func(o *options) { o.errorHandler = errorHandler }
}

// WithDisableKeepalive sets whether keepalive is disabled.
func WithDisableKeepalive(disable bool) Option {
	return func(o *options) { o.disableKeepalive = disable }
}

// WithDisableDefaultDate sets whether the default date header is disabled.
func WithDisableDefaultDate(disable bool) Option {
	return func(o *options) { o.disableDefaultDate = disable }
}

// WithDisableDefaultContentType sets whether the default Content-Type is disabled.
func WithDisableDefaultContentType(disable bool) Option {
	return func(o *options) { o.disableDefaultContentType = disable }
}

// WithDisableHeaderNormalizing sets whether default header normalization is disabled.
func WithDisableHeaderNormalizing(disable bool) Option {
	return func(o *options) { o.disableHeaderNormalizing = disable }
}

// WithStreamRequestBody sets whether the request body is streamed.
func WithStreamRequestBody(enable bool) Option {
	return func(o *options) { o.streamRequestBody = enable }
}

// WithDisablePreParseMultipartForm sets whether pre-parsing of multipart/form-data is disabled.
func WithDisablePreParseMultipartForm(disable bool) Option {
	return func(o *options) { o.disablePreParseMultipartForm = disable }
}

// WithReduceMemoryUsage sets whether memory usage is reduced.
func WithReduceMemoryUsage(enable bool) Option {
	return func(o *options) { o.reduceMemoryUsage = enable }
}

// WithEnableIPValidation sets whether IP validation is enabled.
func WithEnableIPValidation(enable bool) Option {
	return func(o *options) { o.enableIPValidation = enable }
}

// WithEnableSplittingOnParsers sets whether splitting the request body on parsers is enabled.
func WithEnableSplittingOnParsers(enable bool) Option {
	return func(o *options) { o.enableSplittingOnParsers = enable }
}
