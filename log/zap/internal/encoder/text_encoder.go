/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 8:50 下午
 * @Desc: TODO
 */

package encoder

import (
	"fmt"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type TextEncoder struct {
	zapcore.ObjectEncoder
	bufferPool      buffer.Pool
	timestampFormat string
	CallerFullPath  bool
}

var _ zapcore.Encoder = &TextEncoder{}

func NewTextEncoder(timestampFormat string, callerFullPath bool) zapcore.Encoder {
	return &TextEncoder{
		bufferPool:      buffer.NewPool(),
		timestampFormat: timestampFormat,
	}
}

func (e *TextEncoder) Clone() zapcore.Encoder {
	return nil
}

func (e *TextEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := e.bufferPool.Get()

	line.AppendString(ent.Level.CapitalString()[0:4])
	line.AppendString(fmt.Sprintf("[%s]", ent.Time.Format(e.timestampFormat)))
	line.AppendString("\n")

	return line, nil
}
