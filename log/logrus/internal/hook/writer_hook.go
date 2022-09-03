/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/3 6:48 下午
 * @Desc: TODO
 */

package hook

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var _ logrus.Hook = NewWriterHook()

type WriterHook struct {
	logger *logrus.Logger
}

func NewWriterHook() *WriterHook {
	return &WriterHook{}
}

func (h *WriterHook) Levels() []logrus.Level {
	return nil
}

func (h *WriterHook) Fire(entry *logrus.Entry) error {
	fmt.Println(entry.Level)
	h.logger.IsLevelEnabled(entry.Level)

	return nil
}
