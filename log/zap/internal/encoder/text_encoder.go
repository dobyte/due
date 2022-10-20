/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 8:50 下午
 * @Desc: TODO
 */

package encoder

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type TextEncoder struct {
	zapcore.ObjectEncoder
	bufferPool     buffer.Pool
	timeFormat     string
	callerFullPath bool
	isTerminal     bool
}

const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

var _ zapcore.Encoder = &TextEncoder{}

func NewTextEncoder(timeFormat string, callerFullPath, isTerminal bool) zapcore.Encoder {
	return &TextEncoder{
		bufferPool:     buffer.NewPool(),
		timeFormat:     timeFormat,
		callerFullPath: callerFullPath,
		isTerminal:     isTerminal,
	}
}

func (e *TextEncoder) Clone() zapcore.Encoder {
	return nil
}

func (e *TextEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := e.bufferPool.Get()

	levelText := ent.Level.CapitalString()[0:4]
	if e.isTerminal {
		var levelColor int
		switch ent.Level {
		case zapcore.DebugLevel:
			levelColor = gray
		case zapcore.WarnLevel:
			levelColor = yellow
		case zapcore.ErrorLevel, zapcore.FatalLevel, zapcore.PanicLevel:
			levelColor = red
		case zapcore.InfoLevel:
			levelColor = blue
		default:
			levelColor = blue
		}
		line.AppendString(fmt.Sprintf("\x1b[%dm%s", levelColor, levelText))
		line.AppendString(fmt.Sprintf("\x1b[0m[%s]", ent.Time.Format(e.timeFormat)))
	} else {
		line.AppendString(levelText)
		line.AppendString(fmt.Sprintf("[%s]", ent.Time.Format(e.timeFormat)))
	}

	if ent.Caller.Defined {
		if e.callerFullPath {
			line.AppendString(fmt.Sprintf(" %s:%d ", ent.Caller.File, ent.Caller.Line))
		} else {
			_, file := filepath.Split(ent.Caller.File)
			line.AppendString(fmt.Sprintf(" %s:%d ", file, ent.Caller.Line))
		}
	}

	line.AppendString(strings.TrimSuffix(ent.Message, "\n"))

	if ent.Stack != "" {
		line.AppendByte('\n')
		line.AppendString("Stack:\n")

		stacks := strings.Split(ent.Stack, "\n")
		for i := range stacks {
			if i%2 == 0 {
				stacks[i] = strconv.Itoa(i/2+1) + ". " + stacks[i]
			}
		}
		line.AppendString(strings.Join(stacks, "\n"))
	}

	line.AppendString("\n")

	return line, nil
}
