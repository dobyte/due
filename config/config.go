package config

import (
	"context"
	"github.com/dobyte/due/v2/config/configurator"
	"github.com/dobyte/due/v2/config/file"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/log"
)

var globalConfigurator configurator.Configurator

func SetDefaultConfigurator() {
	SetConfigurator(NewConfigurator(file.NewSource()))
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

// NewConfigurator 新建配置器
func NewConfigurator(sources ...configurator.Source) configurator.Configurator {
	return configurator.NewConfigurator(configurator.WithSources(sources...))
}

// SetSource 通过设置配置源来设置配置器
func SetSource(sources ...configurator.Source) {
	SetConfigurator(configurator.NewConfigurator(configurator.WithSources(sources...)))
}

// Has 检测多个匹配规则中是否存在配置
func Has(pattern string) bool {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the has operation will be ignored.")
		return false
	}

	return globalConfigurator.Has(pattern)
}

// Get 获取配置值
func Get(pattern string, def ...interface{}) value.Value {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the get operation will be ignored.")
		return value.NewValue()
	}

	return globalConfigurator.Get(pattern, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) error {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the set operation will be ignored.")
		return nil
	}

	return globalConfigurator.Set(pattern, value)
}

// Match 匹配多个规则
func Match(patterns ...string) configurator.Matcher {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the gets operation will be ignored.")
		return configurator.NewEmptyMatcher()
	}

	return globalConfigurator.Match(patterns...)
}

// Watch 设置监听回调
func Watch(cb configurator.WatchCallbackFunc, names ...string) {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the watch operation will be ignored.")
		return
	}

	globalConfigurator.Watch(cb, names...)
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
func Store(ctx context.Context, source string, file string, content interface{}, override ...bool) error {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the store operation will be ignored.")
		return nil
	}

	return globalConfigurator.Store(ctx, source, file, content, override...)
}

// Close 关闭配置监听
func Close() {
	if globalConfigurator == nil {
		log.Warn("the configurator component is not injected, and the close operation will be ignored.")
	}

	globalConfigurator.Close()
}
