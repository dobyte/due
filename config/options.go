package config

import (
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

type Decoder func(configuration *Configuration) (map[string]interface{}, error)

type options struct {
	sources []Source
	decoder Decoder
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
func defaultDecoder(c *Configuration) (map[string]interface{}, error) {
	var name string
	switch strings.ToLower(c.Format) {
	case json.Name, xml.Name, proto.Name, toml.Name, yaml.Name:
		name = c.Format
	case "yml":
		name = yaml.Name
	default:
		return nil, errors.New("invalid encoding format")
	}

	dest := make(map[string]interface{})
	err := encoding.Invoke(name).Unmarshal(c.Content, &dest)
	if err != nil {
		return nil, err
	}

	return dest, nil
}
