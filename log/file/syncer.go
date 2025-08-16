package file

import (
	"github.com/dobyte/due/v2/log/internal"
)

const Name = "file"

type Syncer struct {
	opts *options
}

func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Syncer{}
	s.opts = o

	return s
}

// Name 同步器名称
func (s *Syncer) Name() string {
	return Name
}

// Write 写入日志
func (s *Syncer) Write(entity *internal.Entity) error {
	return nil
}

// Close 关闭同步器
func (s *Syncer) Close() error {
	return nil
}
