package config

import (
	"context"
	"github.com/dobyte/due/v2/core/value"
)

var globalConfigurator Configurator

// SetConfigurator 设置配置器
func SetConfigurator(configurator Configurator) {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}
	globalConfigurator = configurator
}

// GetConfigurator 获取配置器
func GetConfigurator() Configurator {
	return globalConfigurator
}

// SetConfiguratorWithSources 通过设置配置源来设置配置器
func SetConfiguratorWithSources(sources ...Source) {
	SetConfigurator(NewConfigurator(WithSources(sources...)))
}

// Has 检测多个匹配规则中是否存在配置
func Has(pattern string) bool {
	if globalConfigurator == nil {
		return false
	}

	return globalConfigurator.Has(pattern)
}

// Get 获取配置值
func Get(pattern string, def ...interface{}) value.Value {
	if globalConfigurator == nil {
		return value.NewValue()
	}

	return globalConfigurator.Get(pattern, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) error {
	if globalConfigurator == nil {
		return nil
	}

	return globalConfigurator.Set(pattern, value)
}

// Match 匹配多个规则
func Match(patterns ...string) Matcher {
	if globalConfigurator == nil {
		return newEmptyMatcher()
	}

	return globalConfigurator.Match(patterns...)
}

// Watch 设置监听回调
func Watch(cb WatchCallbackFunc, names ...string) {
	if globalConfigurator == nil {
		return
	}

	globalConfigurator.Watch(cb, names...)
}

// Load 加载配置项
func Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	if globalConfigurator == nil {
		return nil, nil
	}

	return globalConfigurator.Load(ctx, source, file...)
}

// Store 保存配置项
func Store(ctx context.Context, source string, file string, content interface{}, override ...bool) error {
	if globalConfigurator == nil {
		return nil
	}

	return globalConfigurator.Store(ctx, source, file, content, override...)
}

// Close 关闭配置监听
func Close() {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}
}
