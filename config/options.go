package config

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"github.com/BurntSushi/toml"
	"github.com/symsimmy/due/errors"
	"gopkg.in/yaml.v3"
	"strings"
)

type Option func(o *options)

type Decoder func(configuration *Configuration, value interface{}) error

type options struct {
	ctx           context.Context
	sources       []Source
	remoteSources []string
	remoteReaders []interface{}
	decoder       Decoder
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

// WithRemoteSources 设置远程配置源
func WithRemoteSources(remoteSources ...string) Option {
	return func(o *options) { o.remoteSources = remoteSources[:] }
}

// 默认解码器
func defaultDecoder(c *Configuration, value interface{}) error {
	switch strings.ToLower(c.Format) {
	case "json":
		return unmarshal(c.Content, value, json.Unmarshal)
	case "xml":
		return unmarshal(c.Content, value, xml.Unmarshal)
	case "yaml", "yml":
		return unmarshal(c.Content, value, yaml.Unmarshal)
	case "toml":
		return unmarshal(c.Content, value, toml.Unmarshal)
	default:
		return errors.New("invalid encoding format")
	}
}

func unmarshal(content []byte, value interface{}, fn func(data []byte, v interface{}) error) (err error) {
	if err = fn(content, value); err == nil {
		return
	}

	value = make([]interface{}, 0)
	err = fn(content, value)

	return
}
