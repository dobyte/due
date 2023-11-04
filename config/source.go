package config

import "context"

const (
	ReadOnly  Mode = "read-only"  // 只读
	WriteOnly Mode = "write-only" // 只写
	ReadWrite Mode = "read-write" // 读写
)

type Mode string

type Source interface {
	// Name 配置源名称
	Name() string
	// Load 加载配置项
	Load(ctx context.Context, file ...string) ([]*Configuration, error)
	// Store 保存配置项
	Store(ctx context.Context, file string, content []byte) error
	// Watch 监听配置项
	Watch(ctx context.Context) (Watcher, error)
	// Close 关闭配置源
	Close() error
}

type Watcher interface {
	// Next 返回配置列表
	Next() ([]*Configuration, error)
	// Stop 停止监听
	Stop() error
}
