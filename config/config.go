package config

import (
	"context"
	"github.com/dobyte/due/v2/config/configurator"
	"github.com/dobyte/due/v2/config/file"
	"github.com/dobyte/due/v2/internal/value"
	"github.com/dobyte/due/v2/log"
)

var globalConfigurator configurator.Configurator

// SetDefaultConfigurator 设置默认的文件配置器
func SetDefaultConfigurator() {
	SetConfigurator(configurator.NewConfigurator(configurator.WithSources(file.NewSource())))
}

// SetConfigurator 设置配置器
func SetConfigurator(configurator configurator.Configurator) {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}
	globalConfigurator = configurator
}

// GetConfigurator 获取配置器
func GetConfigurator() configurator.Configurator {
	return globalConfigurator
}

// Has 检测多个匹配规则中是否存在配置
func Has(patterns ...string) bool {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the has operation will be ignored.")
		return false
	}

	return globalConfigurator.Has(patterns...)
}

// Get 获取配置值
func Get(pattern string, def ...interface{}) value.Value {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the get operation will be ignored.")
		return value.NewValue()
	}

	return globalConfigurator.Get(pattern, def...)
}

// Gets 获取多个匹配规则中的配置值
func Gets(patterns []string, def ...interface{}) value.Value {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the gets operation will be ignored.")
		return value.NewValue()
	}

	return globalConfigurator.Gets(patterns, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) error {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the set operation will be ignored.")
		return nil
	}

	return globalConfigurator.Set(pattern, value)
}

// Watch 设置监听回调
func Watch(cb configurator.WatchCallbackFunc) {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the watch operation will be ignored.")
		return
	}

	globalConfigurator.Watch(cb)
}

// Load 加载配置项
func Load(ctx context.Context, source string, file ...string) ([]*configurator.Configuration, error) {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the load operation will be ignored.")
		return nil, nil
	}

	return globalConfigurator.Load(ctx, source, file...)
}

// Store 保存配置项
func Store(ctx context.Context, source string, name string, content interface{}) error {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the store operation will be ignored.")
		return nil
	}

	return globalConfigurator.Store(ctx, source, name, content)
}

// Close 关闭配置监听
func Close() {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the close operation will be ignored.")
	}

	globalConfigurator.Close()
}
