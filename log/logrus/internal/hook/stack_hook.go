/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/7 11:20 上午
 * @Desc: TODO
 */

package hook

import (
	"github.com/dobyte/due/log/logrus/v2/internal/define"
	"github.com/dobyte/due/v2/core/stack"
	"github.com/dobyte/due/v2/log"
	"github.com/sirupsen/logrus"
)

type StackHook struct {
	stackLevel log.Level
	callerSkip int
}

func NewStackHook(stackLevel log.Level, callerSkip int) *StackHook {
	return &StackHook{stackLevel: stackLevel, callerSkip: callerSkip}
}

func (h *StackHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *StackHook) Fire(entry *logrus.Entry) error {
	var level log.Level
	switch entry.Level {
	case logrus.DebugLevel:
		level = log.DebugLevel
	case logrus.InfoLevel:
		level = log.InfoLevel
	case logrus.WarnLevel:
		level = log.WarnLevel
	case logrus.ErrorLevel:
		level = log.ErrorLevel
	case logrus.FatalLevel:
		level = log.FatalLevel
	case logrus.PanicLevel:
		level = log.PanicLevel
	}

	depth := stack.First
	if _, ok := entry.Data[define.StackOutFlagField]; ok {
		if h.stackLevel != log.NoneLevel && level >= h.stackLevel {
			depth = stack.Full
		} else {
			delete(entry.Data, define.StackOutFlagField)
		}
	}

	st := stack.Callers(8+h.callerSkip, depth)
	defer st.Free()
	entry.Data[define.StackFramesFlagField] = st.Frames()

	return nil
}
