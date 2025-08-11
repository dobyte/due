package config

import (
	"github.com/dobyte/due/v2/core/value"
)

type Matcher interface {
	// Has 检测多个匹配规则中是否存在配置
	Has() bool
	// Get 获取配置值
	Get(def ...any) value.Value
	// Scan 扫描读取配置值
	Scan(dest any) error
}

type defaultMatcher struct {
	c        *defaultConfigurator
	patterns []string
}

func newEmptyMatcher() Matcher {
	return &defaultMatcher{}
}

// Has 是否存在配置
func (m *defaultMatcher) Has() bool {
	if m.c == nil {
		return false
	}

	for _, pattern := range m.patterns {
		if ok := m.c.doHas(pattern); ok {
			return ok
		}
	}

	return false
}

// Get 获取配置值
func (m *defaultMatcher) Get(def ...any) value.Value {
	if m.c != nil {
		for _, pattern := range m.patterns {
			if val, ok := m.c.doGet(pattern); ok {
				return val
			}
		}
	}

	return value.NewValue(def...)
}

// Scan 扫描读取配置值
func (m *defaultMatcher) Scan(dest any) error {
	if m.c != nil {
		for _, pattern := range m.patterns {
			if val, ok := m.c.doGet(pattern); ok {
				return val.Scan(dest)
			}
		}
	}

	return nil
}
