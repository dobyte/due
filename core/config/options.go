package config

import (
	"context"
	"log"
	"slices"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/encoding/toml"
	"github.com/dobyte/due/v2/encoding/xml"
	"github.com/dobyte/due/v2/encoding/yaml"
	"github.com/dobyte/due/v2/errors"
)

// Option configures a configurator.
type Option func(o *options)

// Logger is the logging interface used by a configurator. A logger with level-specific methods,
// such as the framework logger, satisfies it.
type Logger interface {
	// Infof writes a formatted info-level log record.
	Infof(format string, a ...any)
	// Warnf writes a formatted warn-level log record.
	Warnf(format string, a ...any)
	// Errorf writes a formatted error-level log record.
	Errorf(format string, a ...any)
}

// defaultLogger writes every record to the standard logger.
type defaultLogger struct{}

func (defaultLogger) Infof(format string, a ...any)  { log.Printf(format, a...) }
func (defaultLogger) Warnf(format string, a ...any)  { log.Printf(format, a...) }
func (defaultLogger) Errorf(format string, a ...any) { log.Printf(format, a...) }

// Encoder encodes content in the given format into bytes.
type Encoder func(format string, content any) ([]byte, error)

// Decoder decodes content in the given format.
type Decoder func(format string, content []byte) (any, error)

// Scanner scans content in the given format into dest.
type Scanner func(format string, content []byte, dest any) error

type options struct {
	ctx     context.Context
	logger  Logger
	sources []Source
	encoder Encoder
	decoder Decoder
	scanner Scanner
}

func defaultOptions() *options {
	return &options{
		ctx:     context.Background(),
		logger:  defaultLogger{},
		encoder: defaultEncoder,
		decoder: defaultDecoder,
		scanner: defaultScanner,
	}
}

// WithContext sets the base context of the configurator.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithLogger sets the logger used to report config source, watcher and callback events. The
// configurator does not close the given logger.
func WithLogger(logger Logger) Option {
	return func(o *options) { o.logger = logger }
}

// WithSources sets the config sources. The given slice is copied, so later changes to it do not
// affect the configurator.
func WithSources(sources ...Source) Option {
	cloned := slices.Clone(sources)

	return func(o *options) { o.sources = cloned }
}

// WithEncoder sets the encoder used when storing config content.
func WithEncoder(encoder Encoder) Option {
	return func(o *options) { o.encoder = encoder }
}

// WithDecoder sets the decoder used when loading config content.
func WithDecoder(decoder Decoder) Option {
	return func(o *options) { o.decoder = decoder }
}

// WithScanner sets the scanner used when scanning config content into a destination.
func WithScanner(scanner Scanner) Option {
	return func(o *options) { o.scanner = scanner }
}

// normalize restores the defaults of options that were explicitly set to nil, so that a nil option
// cannot cause a nil dereference later.
func (o *options) normalize() {
	if o.ctx == nil {
		o.ctx = context.Background()
	}

	if o.logger == nil {
		o.logger = defaultLogger{}
	}

	if o.encoder == nil {
		o.encoder = defaultEncoder
	}

	if o.decoder == nil {
		o.decoder = defaultDecoder
	}

	if o.scanner == nil {
		o.scanner = defaultScanner
	}
}

// defaultEncoder is the default [Encoder]; it supports JSON, XML, YAML and TOML.
func defaultEncoder(format string, content any) ([]byte, error) {
	switch strings.ToLower(format) {
	case json.Name:
		return json.Marshal(content)
	case xml.Name:
		return xml.Marshal(content)
	case yaml.Name, yaml.ShortName:
		return yaml.Marshal(content)
	case toml.Name:
		return toml.Marshal(content)
	default:
		return nil, errors.ErrInvalidFormat
	}
}

// defaultDecoder is the default [Decoder]; it supports JSON, XML, YAML and TOML.
func defaultDecoder(format string, content []byte) (any, error) {
	switch strings.ToLower(format) {
	case json.Name:
		return unmarshal(content, json.Unmarshal)
	case xml.Name:
		return unmarshal(content, xml.Unmarshal)
	case yaml.Name, yaml.ShortName:
		return unmarshal(content, yaml.Unmarshal)
	case toml.Name:
		return unmarshal(content, toml.Unmarshal)
	default:
		return nil, errors.ErrInvalidFormat
	}
}

// defaultScanner is the default [Scanner]; it supports JSON, XML, YAML and TOML.
func defaultScanner(format string, content []byte, dest any) error {
	switch strings.ToLower(format) {
	case json.Name:
		return json.Unmarshal(content, dest)
	case xml.Name:
		return xml.Unmarshal(content, dest)
	case yaml.Name, yaml.ShortName:
		return yaml.Unmarshal(content, dest)
	case toml.Name:
		return toml.Unmarshal(content, dest)
	default:
		return errors.ErrInvalidFormat
	}
}

func unmarshal(content []byte, fn func(data []byte, v any) error) (dest any, err error) {
	dest = make(map[string]any)
	if err = fn(content, &dest); err == nil {
		return
	}

	dest = make([]any, 0)
	err = fn(content, &dest)

	return
}
