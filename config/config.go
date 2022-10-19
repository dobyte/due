package config

import (
	"flag"
	"github.com/dobyte/due/env"
	"github.com/dobyte/due/internal/value"
)

const (
	dueConfigEnvName  = "DUE_CONFIG"
	defaultConfigPath = "./config"
)

var globalReader Reader

func init() {
	def := flag.String("config", defaultConfigPath, "Specify the configuration file path")
	path := env.Get(dueConfigEnvName, *def).String()
	SetReader(NewReader(WithSources(NewSource(path))))
}

// SetReader 设置配置读取器
func SetReader(reader Reader) {
	globalReader = reader
}

// GetReader 获取配置读取器
func GetReader() Reader {
	return globalReader
}

// Get 获取配置值
func Get(pattern string, def ...interface{}) value.Value {
	return globalReader.Get(pattern, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) error {
	return globalReader.Set(pattern, value)
}

// Close 关闭配置读取器
func Close() {
	globalReader.Close()
}
