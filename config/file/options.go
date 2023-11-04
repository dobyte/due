package file

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath = "./config"
	defaultMode = config.ReadOnly
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
	// 支持read-only、write-only和read-write三种模式，默认为read-only模式
	mode config.Mode
}

func defaultOptions() *options {
	return &options{
		path: etc.Get(defaultPathKey, defaultPath).String(),
		mode: config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
	}
}

// WithPath 设置配置文件或配置目录路径
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode 设置读写模式
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}
