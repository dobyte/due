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
	"runtime"
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

	levelText := strings.ToUpper(entry.Level.String())[0:4]

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

		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s]", levelColor, levelText, entry.Time.Format(f.TimestampFormat))
	} else {
		entry.Data[defaultOutFileFlag] = true
		fmt.Fprintf(b, "%s[%s]", levelText, entry.Time.Format(f.TimestampFormat))
	}

	var frames []runtime.Frame
	if v, ok := entry.Data["frames"]; ok {
		frames = v.([]runtime.Frame)
	}

	if len(frames) > 0 {
		fmt.Fprintf(b, " %s", f.framesToCaller(frames))
	}

	message := strings.TrimSuffix(entry.Message, "\n")
	if message != "" {
		fmt.Fprintf(b, " %s", message)
	}

	if _, ok := entry.Data["stack_out"]; ok && len(frames) > 0 {
		fmt.Fprint(b, "\nStack:")
		for i, frame := range frames {
			fmt.Fprintf(b, "\n%d.%s\n", i+1, frame.Function)
			fmt.Fprintf(b, "\t%s:%d", frame.File, frame.Line)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *TextFormatter) framesToCaller(frames []runtime.Frame) string {
	if len(frames) == 0 {
		return ""
	}

	file := frames[0].File
	if !f.CallerFullPath {
		_, file = filepath.Split(file)
	}

	return fmt.Sprintf("%s:%d", file, frames[0].Line)
}
