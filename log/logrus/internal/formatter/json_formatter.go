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
	"github.com/dobyte/due/log/logrus/v2/internal/define"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"runtime"
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

type JsonFormatter struct {
	TimeFormat     string
	CallerFullPath bool
}

// Format renders a single log entry
func (f *JsonFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	levelText := strings.ToUpper(entry.Level.String())[0:4]

	fmt.Fprintf(b, `{"%s":"%s"`, fieldKeyLevel, levelText)
	fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyTime, entry.Time.Format(f.TimeFormat))

	var frames []runtime.Frame
	if v, ok := entry.Data[define.StackFramesFlagField]; ok {
		frames = v.([]runtime.Frame)
	}

	if len(frames) > 0 {
		fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyFile, f.framesToCaller(frames))
	}

	message := strings.TrimSuffix(entry.Message, "\n")
	if message != "" {
		fmt.Fprintf(b, `,"%s":"%s"`, fieldKeyMsg, message)
	}

	if _, ok := entry.Data[define.StackOutFlagField]; ok && len(frames) > 0 {
		fmt.Fprintf(b, `,"%s":[`, fieldKeyStack)
		for i, frame := range frames {
			if i == 0 {
				fmt.Fprintf(b, `{"%s":"%s"`, fieldKeyStackFunc, frame.Function)
			} else {
				fmt.Fprintf(b, `,{"%s":"%s"`, fieldKeyStackFunc, frame.Function)
			}
			fmt.Fprintf(b, `,"%s":"%s:%d"}`, fieldKeyStackFile, frame.File, frame.Line)
		}
		fmt.Fprint(b, "]")
	}

	fmt.Fprint(b, "}\n")
	return b.Bytes(), nil
}

func (f *JsonFormatter) framesToCaller(frames []runtime.Frame) string {
	if len(frames) == 0 {
		return ""
	}

	file := frames[0].File
	if !f.CallerFullPath {
		_, file = filepath.Split(file)
	}

	return fmt.Sprintf("%s:%d", file, frames[0].Line)
}
