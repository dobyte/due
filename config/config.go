package config

import (
	"github.com/symsimmy/due/env"
	"github.com/symsimmy/due/flag"
	"github.com/symsimmy/due/common/value"
)

const (
	dueConfigArgName  = "config"
	dueConfigEnvName  = "DUE_CONFIG"
	defaultConfigPath = "./config"
)

var globalReader Reader

func init() {
	path := flag.String(dueConfigArgName, defaultConfigPath)
	path = env.Get(dueConfigEnvName, path).String()
	SetReader(NewReader(WithSources(NewSource(path))))
}

// SetReader 设置配置读取器
func SetReader(reader Reader) {
	if globalReader != nil {
		globalReader.Close()
	}
	globalReader = reader
}

// GetReader 获取配置读取器
func GetReader() Reader {
	return globalReader
}

// Has 是否存在配置
func Has(pattern string) bool {
	return globalReader.Has(pattern)
}

// Get 获取配置值
func Get(pattern string, def ...interface{}) value.Value {
	return globalReader.Get(pattern, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) error {
	return globalReader.Set(pattern, value)
}

// Close 关闭配置监听
func Close() {
	if globalReader != nil {
		globalReader.Close()
	}
}
