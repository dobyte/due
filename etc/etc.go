package etc

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/file/core"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/env"
	"github.com/dobyte/due/v2/flag"
)

// etc主要被当做项目启动配置存在；常用于集群配置、服务组件配置等。
// etc只能通过配置文件进行配置；并且无法通过master管理服进行修改。
// 如想在业务使用配置，推荐使用config配置中心进行实现。
// config配置中心的配置信息可通过master管理服进行动态修改。

const (
	dueEtcArgName  = "etc"
	dueEtcEnvName  = "DUE_ETC"
	defaultEtcPath = "./etc"
)

var globalConfigurator config.Configurator

func init() {
	path := flag.String(dueEtcArgName, defaultEtcPath)
	path = env.Get(dueEtcEnvName, path).String()
	globalConfigurator = config.NewConfigurator(config.WithSources(core.NewSource(path, config.ReadOnly)))
}

// SetConfigurator 设置配置器
func SetConfigurator(configurator config.Configurator) {
	if globalConfigurator != nil {
		globalConfigurator.Close()
	}

	globalConfigurator = configurator
}

// GetConfigurator 获取配置器
func GetConfigurator() config.Configurator {
	return globalConfigurator
}

// Has 是否存在配置
func Has(pattern string) bool {
	return globalConfigurator.Has(pattern)
}

// Get 获取配置值
func Get(pattern string, def ...interface{}) value.Value {
	return globalConfigurator.Get(pattern, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) error {
	return globalConfigurator.Set(pattern, value)
}

// Match 匹配多个规则
func Match(patterns ...string) config.Matcher {
	return globalConfigurator.Match(patterns...)
}

// Close 关闭配置监听
func Close() {
	globalConfigurator.Close()
}
