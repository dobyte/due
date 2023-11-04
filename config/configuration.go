package config

import "github.com/dobyte/due/v2/errors"

// Configuration 配置项
type Configuration struct {
	decoder  Decoder // 解码器
	scanner  Scanner // 扫描器
	Path     string  // 文件路径
	File     string  // 文件全称
	Name     string  // 文件名称
	Format   string  // 文件格式
	Content  []byte  // 文件内容
	FullPath string  // 文件全路径
}

// Decode 解码
func (c *Configuration) Decode() (interface{}, error) {
	if c.decoder == nil {
		return nil, errors.ErrInvalidDecoder
	}

	return c.decoder(c.Format, c.Content)
}

// Scan 扫描
func (c *Configuration) Scan(dest interface{}) error {
	if c.scanner == nil {
		return errors.ErrInvalidScanner
	}

	return c.scanner(c.Format, c.Content, dest)
}
