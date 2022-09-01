/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 11:21 上午
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
	FieldKeyLevel = logrus.FieldKeyLevel
	FieldKeyTime  = logrus.FieldKeyTime
	FieldKeyFile  = logrus.FieldKeyFile
	FieldKeyMsg   = logrus.FieldKeyMsg
)

type JsonFormatter struct {
	TimestampFormat string
	CallerFullPath  bool
}

// Format renders a single log entry
func (f *JsonFormatter) Format(entry *logrus.Entry) ([]byte, error) {
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

	if _, ok := entry.Data[defaultOutFileFlag]; len(entry.Logger.Hooks) == 0 || ok {
		entry.Message = strings.TrimSuffix(entry.Message, "\n")
	} else {
		entry.Data[defaultOutFileFlag] = true
		entry.Message = strings.ReplaceAll(strings.TrimSuffix(entry.Message, "\n"), `"`, `\"`)
	}

	_, _ = fmt.Fprintf(b,
		`{"%s":"%s","%s":"%s","%s":"%s","%s":"%s"}`,
		FieldKeyLevel,
		levelText,
		FieldKeyTime,
		entry.Time.Format(f.TimestampFormat),
		FieldKeyFile,
		caller,
		FieldKeyMsg,
		entry.Message,
	)

	b.WriteByte('\n')
	return b.Bytes(), nil
}
