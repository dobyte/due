package console

import (
	"io"
	"os"

	"github.com/dobyte/due/v2/log/internal"
)

// Name 同步器名称
const Name = "console"

// Syncer 控制台日志同步器
type Syncer struct {
	opts      *options           // 配置项
	writer    io.WriteCloser     // 输出写入器
	formatter internal.Formatter // 日志格式化器
}

// NewSyncer 创建一个控制台日志同步器实例
// @param opts ...Option 可选配置项
// @return @1 *Syncer 同步器实例
func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Syncer{}
	s.opts = o
	s.init()

	return s
}

// init 初始化同步器
func (s *Syncer) init() {
	s.writer = os.Stdout

	if s.opts.format == FormatJson {
		s.formatter = internal.NewJsonFormatter()
	} else {
		s.formatter = internal.NewTextFormatter(true)
	}
}

// Name 同步器名称
// @return @1 string 同步器名称
func (s *Syncer) Name() string {
	return Name
}

// Write 写入日志
// @param entity *internal.Entity 日志实体
// @return @1 error 写入过程中产生的错误
func (s *Syncer) Write(entity *internal.Entity) error {
	buf := s.formatter.Format(entity)
	defer buf.Release()

	data := buf.Bytes()
	for len(data) > 0 {
		n, err := s.writer.Write(data)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrShortWrite
		}

		data = data[n:]
	}

	return nil
}

// Close 关闭同步器
// @return @1 error 关闭过程中产生的错误
func (s *Syncer) Close() error {
	return nil
}
