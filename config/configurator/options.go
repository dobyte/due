package configurator

import (
	"context"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/encoding/toml"
	"github.com/dobyte/due/v2/encoding/xml"
	"github.com/dobyte/due/v2/encoding/yaml"
	"github.com/dobyte/due/v2/errors"
	"strings"
)

type Option func(o *options)

type Encoder func(format string, content interface{}) ([]byte, error)
type Decoder func(format string, content []byte) (interface{}, error)
type Scanner func(format string, content []byte, dest interface{}) error

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

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithSources 设置配置源
func WithSources(sources ...Source) Option {
	return func(o *options) { o.sources = sources[:] }
}

// WithEncoder 设置编码器
func WithEncoder(encoder Encoder) Option {
	return func(o *options) { o.encoder = encoder }
}

// WithDecoder 设置解码器
func WithDecoder(decoder Decoder) Option {
	return func(o *options) { o.decoder = decoder }
}

// 默认编码器
func defaultEncoder(format string, content interface{}) ([]byte, error) {
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
		return nil, errors.New("invalid encoding format")
	}
}

// 默认解码器
func defaultDecoder(format string, content []byte) (interface{}, error) {
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
		return nil, errors.New("invalid decoding format")
	}
}

// 默认扫描器
func defaultScanner(format string, content []byte, dest interface{}) error {
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
		return errors.New("invalid scan format")
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
