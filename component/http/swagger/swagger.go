package swagger

import (
	"github.com/dobyte/due/v2/log"
	"github.com/go-openapi/runtime/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"net/http"
	"os"
	"path"
	"strings"
)

type Config struct {
	Title            string // 文档标题
	FilePath         string // 文档路径
	BasePath         string // 访问路径
	SwaggerBundleUrl string // swagger-ui-bundle.js地址
	SwaggerPresetUrl string // swagger-ui-preset.js地址
	SwaggerStylesUrl string // swagger-ui.css地址
}

const (
	defaultSwaggerBundleUrl = "https://lf26-cdn-tos.bytecdntp.com/cdn/expire-1-M/swagger-ui/4.5.2/swagger-ui-bundle.min.js"
	defaultSwaggerPresetUrl = "https://lf6-cdn-tos.bytecdntp.com/cdn/expire-1-M/swagger-ui/4.5.2/swagger-ui-standalone-preset.min.js"
	defaultSwaggerStylesUrl = "https://lf6-cdn-tos.bytecdntp.com/cdn/expire-1-M/swagger-ui/4.5.2/swagger-ui.min.css"
	swaggerFavicon32Latest  = "https://unpkg.com/swagger-ui-dist/favicon-32x32.png"
	swaggerFavicon16Latest  = "https://unpkg.com/swagger-ui-dist/favicon-16x16.png"
)

func New(cfg Config) fiber.Handler {
	// Verify Swagger file exists
	if _, err := os.Stat(cfg.FilePath); os.IsNotExist(err) {
		log.Fatalf("%s file does not exist", cfg.FilePath)
	}

	// Read Swagger Spec into memory
	rawSpec, err := os.ReadFile(cfg.FilePath)
	if err != nil {
		log.Fatalf("Failed to read provided Swagger file (%s): %v", cfg.FilePath, err)
	}

	// Generate URL path's for the middleware
	specURL := path.Join(cfg.BasePath, cfg.FilePath)
	swaggerUIPath := path.Join("/", cfg.BasePath)

	// Serve the Swagger spec from memory
	swaggerSpecHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".yaml") || strings.HasSuffix(r.URL.Path, ".yml") {
			w.Header().Set("Content-Type", "application/yaml")
			w.Header().Set("Cache-Control", "public, max-age=3600")

			if _, err := w.Write(rawSpec); err != nil {
				http.Error(w, "Error processing YAML Swagger Spec", http.StatusInternalServerError)
				return
			}
		} else if strings.HasSuffix(r.URL.Path, ".json") {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=3600")

			if _, err := w.Write(rawSpec); err != nil {
				http.Error(w, "Error processing JSON Swagger Spec", http.StatusInternalServerError)
				return
			}
		} else {
			http.NotFound(w, r)
		}
	})

	if cfg.SwaggerBundleUrl == "" {
		cfg.SwaggerBundleUrl = defaultSwaggerBundleUrl
	}

	if cfg.SwaggerPresetUrl == "" {
		cfg.SwaggerPresetUrl = defaultSwaggerPresetUrl
	}

	if cfg.SwaggerStylesUrl == "" {
		cfg.SwaggerStylesUrl = defaultSwaggerStylesUrl
	}

	// Create UI middleware
	middlewareHandler := adaptor.HTTPHandler(middleware.SwaggerUI(middleware.SwaggerUIOpts{
		SpecURL:          specURL,
		Path:             cfg.BasePath,
		Title:            cfg.Title,
		SwaggerURL:       cfg.SwaggerBundleUrl,
		SwaggerPresetURL: cfg.SwaggerPresetUrl,
		SwaggerStylesURL: cfg.SwaggerStylesUrl,
	}, swaggerSpecHandler))

	// Return new handler
	return func(c fiber.Ctx) error {
		// Only respond to requests to SwaggerUI and SpecURL (swagger.json)
		if !(c.Path() == swaggerUIPath || c.Path() == specURL) {
			return c.Next()
		}

		// Pass Fiber context to handler
		return middlewareHandler(c)
	}
}
