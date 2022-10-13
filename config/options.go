package config

import (
	"context"
	"github.com/dobyte/due/encoding"
	"github.com/dobyte/due/encoding/json"
	"github.com/dobyte/due/encoding/proto"
	"github.com/dobyte/due/encoding/toml"
	"github.com/dobyte/due/encoding/xml"
	"github.com/dobyte/due/encoding/yaml"
	"github.com/dobyte/due/errors"
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
	var (
		codec encoding.Codec
		dest  interface{}
	)

	switch strings.ToLower(c.Format) {
	case json.Name, xml.Name, proto.Name, toml.Name, yaml.Name:
		codec = encoding.Invoke(c.Format)
	case "yml":
		codec = encoding.Invoke(yaml.Name)
	default:
		return nil, errors.New("invalid encoding format")
	}

	dest = make(map[string]interface{})
	if err := codec.Unmarshal(c.Content, &dest); err == nil {
		return dest, nil
	}

	dest = make([]interface{}, 0)
	if err := codec.Unmarshal(c.Content, &dest); err != nil {
		return nil, err
	}

	return dest, nil
}
