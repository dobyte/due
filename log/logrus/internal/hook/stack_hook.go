/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/7 11:20 上午
 * @Desc: TODO
 */

package hook

import (
	"github.com/sirupsen/logrus"
	"github.com/symsimmy/due/common/stack"
	"github.com/symsimmy/due/log/utils"
)

type StackHook struct {
	stackLevel utils.Level
	callerSkip int
}

func NewStackHook(stackLevel utils.Level, callerSkip int) *StackHook {
	return &StackHook{stackLevel: stackLevel, callerSkip: callerSkip}
}

func (h *StackHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *StackHook) Fire(entry *logrus.Entry) error {
	var level utils.Level
	switch entry.Level {
	case logrus.DebugLevel:
		level = utils.DebugLevel
	case logrus.InfoLevel:
		level = utils.InfoLevel
	case logrus.WarnLevel:
		level = utils.WarnLevel
	case logrus.ErrorLevel:
		level = utils.ErrorLevel
	case logrus.FatalLevel:
		level = utils.FatalLevel
	case logrus.PanicLevel:
		level = utils.PanicLevel
	}

	var depth stack.Depth
	if h.stackLevel != utils.NoneLevel && level >= h.stackLevel {
		depth = stack.Full
		entry.Data["stack_out"] = struct{}{}
	} else {
		depth = stack.First
	}

	st := stack.Callers(8+h.callerSkip, depth)
	defer st.Free()
	entry.Data["stack_frames"] = st.Frames()

	return nil
}
