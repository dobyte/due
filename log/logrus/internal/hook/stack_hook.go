/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/7 11:20 上午
 * @Desc: TODO
 */

package hook

import (
	"github.com/sirupsen/logrus"

	"github.com/dobyte/due/log"
)

const defaultNoneLevel log.Level = 0

type StackHook struct {
	outStackLevel log.Level
}

func NewStackHook(outStackLevel log.Level) *StackHook {
	return &StackHook{outStackLevel: outStackLevel}
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

	if h.outStackLevel != defaultNoneLevel && level >= h.outStackLevel {
		entry.Data["frames"] = log.GetFrames(9, log.StacktraceFull)
	} else {
		entry.Data["frames"] = log.GetFrames(9, log.StacktraceFirst)
	}

	return nil
}
