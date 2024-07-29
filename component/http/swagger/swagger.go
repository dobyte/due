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
	Title    string // 文档标题
	FilePath string // 文档路径
	BasePath string // 访问路径
}

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

	// Define UI Options
	swaggerUIOpts := middleware.SwaggerUIOpts{
		SpecURL: specURL,
		Path:    cfg.BasePath,
		Title:   cfg.Title,
	}

	// Create UI middleware
	middlewareHandler := adaptor.HTTPHandler(middleware.SwaggerUI(swaggerUIOpts, swaggerSpecHandler))

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
