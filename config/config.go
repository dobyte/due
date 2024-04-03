package config

import (
	"fmt"
	"github.com/symsimmy/due/env"
	"github.com/symsimmy/due/flag"
	"github.com/symsimmy/due/value"
)

const (
	dueConfigArgName      = "config"
	dueConfigEnvName      = "DUE_CONFIG"
	defaultConfigPath     = "./config"
	defaultConfigFileName = "default.toml"

	defaultRemoteConfigEnvName = "DUE_REMOTE_CONFIG"
	defaultRemoteConfig        = "apollo"
)

var globalReader Reader

func init() {
	// get path from command line
	path := flag.String(dueConfigArgName, fmt.Sprintf("%v/%v", defaultConfigPath, defaultConfigFileName))

	// overwrite path from env
	var overwritePath string
	localEnv := env.Get(dueConfigEnvName).String()
	if localEnv != "" {
		overwritePath = fmt.Sprintf("%v/%v", defaultConfigPath, localEnv)
	}

	remoteEnv := env.Get(defaultRemoteConfigEnvName, defaultRemoteConfig).String()

	// set local reader
	SetReader(NewReader(WithSources(NewSource(path), NewSource(overwritePath)), WithRemoteSources(remoteEnv)))

	//InitConsulConfig(context.Background())
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
