/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/3 6:48 下午
 * @Desc: TODO
 */

package hook

import (
	"github.com/sirupsen/logrus"
	"io"
	"sync"
)

var _ logrus.Hook = NewWriterHook(nil)

type WriterMap map[logrus.Level]io.Writer

type WriterHook struct {
	mu            sync.Mutex
	writers       WriterMap
	defaultWriter io.Writer
}

func NewWriterHook(output interface{}) *WriterHook {
	h := &WriterHook{}

	switch writer := output.(type) {
	case io.Writer:
		h.defaultWriter = writer
	case WriterMap:
		h.writers = writer
	}

	return h
}

func (h *WriterHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *WriterHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		return err
	}

	for level, writer := range h.writers {
		if !entry.Logger.IsLevelEnabled(level) {
			continue
		}

		if entry.Level > level {
			continue
		}

		if _, err = writer.Write(data); err != nil {
			return err
		}
	}

	if h.defaultWriter != nil {
		if _, err = h.defaultWriter.Write(data); err != nil {
			return err
		}
	}

	return nil
}
