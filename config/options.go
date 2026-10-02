package config

import (
	"context"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/encoding/toml"
	"github.com/dobyte/due/v2/encoding/xml"
	"github.com/dobyte/due/v2/encoding/yaml"
	"github.com/dobyte/due/v2/errors"
)

// Option configures a configurator.
type Option func(o *options)

// Encoder encodes content in the given format into bytes.
type Encoder func(format string, content any) ([]byte, error)

// Decoder decodes content in the given format.
type Decoder func(format string, content []byte) (any, error)

// Scanner scans content in the given format into dest.
type Scanner func(format string, content []byte, dest any) error

type options struct {
	ctx     context.Context
	sources []Source
	encoder Encoder
	decoder Decoder
	scanner Scanner
}

func defaultOptions() *options {
	return &options{
		ctx:     context.Background(),
		encoder: defaultEncoder,
		decoder: defaultDecoder,
		scanner: defaultScanner,
	}
}

// WithContext sets the base context of the configurator.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithSources sets the config sources.
func WithSources(sources ...Source) Option {
	return func(o *options) { o.sources = sources[:] }
}

// WithEncoder sets the encoder used when storing config content.
func WithEncoder(encoder Encoder) Option {
	return func(o *options) { o.encoder = encoder }
}

// WithDecoder sets the decoder used when loading config content.
func WithDecoder(decoder Decoder) Option {
	return func(o *options) { o.decoder = decoder }
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
