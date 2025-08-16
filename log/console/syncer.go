package console

import (
	"io"
	"os"

	"github.com/dobyte/due/v2/log/internal"
)

type Syncer struct {
	opts      *options
	writer    io.WriteCloser
	formatter internal.Formatter
}

func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Syncer{}
	s.opts = o
	s.writer = os.Stdout

	if s.opts.format == FormatJson {
		s.formatter = internal.NewJsonFormatter(true)
	} else {
		s.formatter = internal.NewTextFormatter(true)
	}

	return s
}

// Name 同步器名称
func (s *Syncer) Name() string {
	return "console"
}

// Write 写入日志
func (s *Syncer) Write(entity *internal.Entity) error {
	buf := s.formatter.Format(entity)
	defer buf.Release()

	_, err := s.writer.Write(buf.Bytes())
	return err
}

// Close 关闭同步器
func (s *Syncer) Close() error {
	return s.writer.Close()
}
