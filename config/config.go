package config

var globalReader Reader

func init() {
	SetReader(NewReader())
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
func Get(pattern string, def ...interface{}) *Value {
	return globalReader.Get(pattern, def...)
}

// Set 设置配置值
func Set(pattern string, value interface{}) {
	globalReader.Set(pattern, value)
}

// Close 关闭配置读取器
func Close() {
	globalReader.Close()
}

// Load 加载配置
func Load(name ...string) {
	//return globalReader.Load(name...)
}
