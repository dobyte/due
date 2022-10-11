package config

type Source interface {
	Load() ([]*Configuration, error)
}

// Configuration 配置项
type Configuration struct {
	Name    string
	Format  string
	Content []byte
}
