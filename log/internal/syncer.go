package internal

type Syncer interface {
	// Name 同步器名称
	Name() string
	// Write 写入日志
	Write(entity *Entity) error
	// Close 关闭同步器
	Close() error
}
