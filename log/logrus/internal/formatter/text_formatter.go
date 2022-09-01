/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/31 3:07 下午
 * @Desc: TODO
 */

package formatter

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

const defaultOutFileFlag = "@outFileFlag@"

type TextFormatter struct {
	TimestampFormat string
	CallerFullPath  bool
}

// Format renders a single log entry
func (f *TextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	levelText := strings.ToUpper(entry.Level.String())
	levelText = levelText[0:4]

	caller := ""
	if entry.HasCaller() {
		if f.CallerFullPath {
			caller = fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
		} else {
			_, file := filepath.Split(entry.Caller.File)
			caller = fmt.Sprintf("%s:%d", file, entry.Caller.Line)
		}
	}

	entry.Message = strings.TrimSuffix(entry.Message, "\n")

	if _, ok := entry.Data[defaultOutFileFlag]; len(entry.Logger.Hooks) == 0 || ok {
		var levelColor int
		switch entry.Level {
		case logrus.DebugLevel, logrus.TraceLevel:
			levelColor = gray
		case logrus.WarnLevel:
			levelColor = yellow
		case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
			levelColor = red
		case logrus.InfoLevel:
			levelColor = blue
		default:
			levelColor = blue
		}

		_, _ = fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s] %s %-44s ", levelColor, levelText, entry.Time.Format(f.TimestampFormat), caller, entry.Message)
	} else {
		entry.Data[defaultOutFileFlag] = true
		_, _ = fmt.Fprintf(b, "%s[%s] %s %-44s ", levelText, entry.Time.Format(f.TimestampFormat), caller, entry.Message)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}
