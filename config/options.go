package config

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"github.com/BurntSushi/toml"
	"github.com/dobyte/due/errors"
	"gopkg.in/yaml.v3"
	"strings"
)

type Option func(o *options)

type Decoder func(configuration *Configuration) (interface{}, error)

type options struct {
	ctx     context.Context
	sources []Source
	decoder Decoder
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithSources 设置配置源
func WithSources(sources ...Source) Option {
	return func(o *options) { o.sources = sources[:] }
}

// WithDecoder 设置解码器
func WithDecoder(decoder Decoder) Option {
	return func(o *options) { o.decoder = decoder }
}

// 默认解码器
func defaultDecoder(c *Configuration) (interface{}, error) {
	switch strings.ToLower(c.Format) {
	case "json":
		return unmarshal(c.Content, json.Unmarshal)
	case "xml":
		return unmarshal(c.Content, xml.Unmarshal)
	case "yaml", "yml":
		return unmarshal(c.Content, yaml.Unmarshal)
	case "toml":
		return unmarshal(c.Content, toml.Unmarshal)
	default:
		return nil, errors.New("invalid encoding format")
	}
}

func unmarshal(content []byte, fn func(data []byte, v interface{}) error) (dest interface{}, err error) {
	dest = make(map[string]interface{})
	if err = fn(content, &dest); err == nil {
		return
	}

	dest = make([]interface{}, 0)
	err = fn(content, &dest)

	return
}
