package console

import (
	"io"
	"os"
)

type Syncer struct {
	writer io.WriteCloser
}

func NewSyncer() *Syncer {
	return &Syncer{
		writer: os.Stdout,
	}
}

// Name 同步器名称
func (s *Syncer) Name() string {
	return "console"
}

// Write 写入日志
func (s *Syncer) Write(p []byte) (n int, err error) {
	return s.writer.Write(p)
}

// Close 关闭同步器
func (s *Syncer) Close() error {
	return s.writer.Close()
}
