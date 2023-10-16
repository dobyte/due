package env

import (
	"github.com/dobyte/due/v2/core/value"
	"os"
)

// Get 获取环境变量值
func Get(key string, def ...interface{}) value.Value {
	if val, ok := os.LookupEnv(key); ok {
		return value.NewValue(val)
	}

	return value.NewValue(def...)
}

// Set 设置环境变量值
func Set(key string, value string) error {
	return os.Setenv(key, value)
}

// Del 删除环境变量
func Del(key string) error {
	return os.Unsetenv(key)
}

// Has 是否存在环境变量
func Has(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}
