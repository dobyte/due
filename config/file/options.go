package file

import (
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath = "./config"
	defaultMode = "read-only"
)

const (
	defaultPathKey = "etc.config.file.path"
	defaultModeKey = "etc.config.file.mode"
)

type Option func(o *options)

type options struct {
	// 配置文件或配置目录路径
	path string

	// 读写模式
	// 支持read-only和read-write两种模式，默认为read-only模式
	mode string
}

func defaultOptions() *options {
	return &options{
		path: etc.Get(defaultPathKey, defaultPath).String(),
		mode: etc.Get(defaultModeKey, defaultMode).String(),
	}
}

// WithPath 设置配置文件或配置目录路径
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode 设置读写模式
func WithMode(mode string) Option {
	return func(o *options) { o.mode = mode }
}
