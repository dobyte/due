/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/3 2:01 下午
 * @Desc: TODO
 */

package encoder

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/zap/internal/utils"
)

const (
	fieldKeyLevel     = "level"
	fieldKeyTime      = "time"
	fieldKeyFile      = "file"
	fieldKeyMsg       = "msg"
	fieldKeyStack     = "stack"
	fieldKeyStackFunc = "func"
	fieldKeyStackFile = "file"
)

type JsonEncoder struct {
	zapcore.ObjectEncoder
	bufferPool      buffer.Pool
	timestampFormat string
	callerFormat    log.CallerFormat
}

func NewJsonEncoder(timestampFormat string, callerFormat log.CallerFormat) zapcore.Encoder {
	return &JsonEncoder{
		bufferPool:      buffer.NewPool(),
		timestampFormat: timestampFormat,
		callerFormat:    callerFormat,
	}
}

func (e *JsonEncoder) Clone() zapcore.Encoder {
	return nil
}

func (e *JsonEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := e.bufferPool.Get()
	line.AppendByte('{')
	line.AppendString(fmt.Sprintf(`"%s":"%s"`, fieldKeyLevel, ent.Level.CapitalString()))
	line.AppendString(fmt.Sprintf(`,"%s":"%s"`, fieldKeyTime, ent.Time.Format(e.timestampFormat)))

	if ent.Caller.Defined {
		var file string
		if e.callerFormat == log.CallerFullPath {
			file = ent.Caller.File
		} else {
			_, file = filepath.Split(ent.Caller.File)
		}
		line.AppendString(fmt.Sprintf(`,"%s":"%s"`, fieldKeyFile, fmt.Sprintf("%s:%d", file, ent.Caller.Line)))
	}

	line.AppendString(fmt.Sprintf(`,"%s":"%s"`, fieldKeyMsg, utils.Addslashes(strings.TrimSuffix(ent.Message, "\n"))))

	if ent.Stack != "" {
		line.AppendString(fmt.Sprintf(`,"%s":[`, fieldKeyStack))

		stacks := strings.Split(ent.Stack, "\n")
		for i := range stacks {
			if i%2 == 0 {
				if i/2 == 0 {
					line.AppendString(fmt.Sprintf(`{"%s":"%s"`, fieldKeyStackFunc, stacks[i]))
				} else {
					line.AppendString(fmt.Sprintf(`,{"%s":"%s"`, fieldKeyStackFunc, stacks[i]))
				}
			} else {
				line.AppendString(fmt.Sprintf(`,"%s":"%s"}`, fieldKeyStackFile, strings.TrimLeft(stacks[i], "\t")))
			}
		}
		line.AppendByte(']')
	}

	line.AppendByte('}')
	line.AppendString("\n")

	return line, nil
}
