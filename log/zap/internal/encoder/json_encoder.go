/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/3 2:01 下午
 * @Desc: TODO
 */

package encoder

import (
	"fmt"
	"github.com/dobyte/due/log/zap/v2/internal/utils"
	"github.com/dobyte/due/v2/utils/xconv"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"path/filepath"
	"strings"
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

const StackFlag = "_stack"

type JsonEncoder struct {
	zapcore.ObjectEncoder
	bufferPool     buffer.Pool
	timeFormat     string
	callerFullPath bool
}

func NewJsonEncoder(timeFormat string, callerFullPath bool) zapcore.Encoder {
	return &JsonEncoder{
		bufferPool:     buffer.NewPool(),
		timeFormat:     timeFormat,
		callerFullPath: callerFullPath,
	}
}

func (e *JsonEncoder) Clone() zapcore.Encoder {
	return nil
}

func (e *JsonEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := e.bufferPool.Get()
	stack := false

	if len(fields) > 0 && fields[0].Key == StackFlag && fields[0].Type == zapcore.BoolType {
		stack = xconv.Bool(fields[0].Integer)
	}

	var levelText string
	switch ent.Level {
	case zapcore.DPanicLevel:
		levelText = zapcore.PanicLevel.CapitalString()
	default:
		levelText = ent.Level.CapitalString()
	}

	line.AppendByte('{')
	line.AppendString(fmt.Sprintf(`"%s":"%s"`, fieldKeyLevel, levelText))
	line.AppendString(fmt.Sprintf(`,"%s":"%s"`, fieldKeyTime, ent.Time.Format(e.timeFormat)))

	if ent.Caller.Defined {
		var file string
		if e.callerFullPath {
			file = ent.Caller.File
		} else {
			_, file = filepath.Split(ent.Caller.File)
		}
		line.AppendString(fmt.Sprintf(`,"%s":"%s"`, fieldKeyFile, fmt.Sprintf("%s:%d", file, ent.Caller.Line)))
	}

	line.AppendString(fmt.Sprintf(`,"%s":"%s"`, fieldKeyMsg, utils.Addslashes(strings.TrimSuffix(ent.Message, "\n"))))

	if stack && ent.Stack != "" {
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
				line.AppendString(fmt.Sprintf(`,"%s":"%s"}`, fieldKeyStackFile, strings.TrimPrefix(stacks[i], "\t")))
			}
		}
		line.AppendByte(']')
	}

	line.AppendByte('}')
	line.AppendString("\n")

	return line, nil
}
