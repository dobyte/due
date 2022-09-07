/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/30 5:36 下午
 * @Desc: TODO
 */

package logrus_test

import (
	"testing"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/logrus"
)

func TestNewLogger(t *testing.T) {
	l := logrus.NewLogger(
		logrus.WithOutFile("./log/due.log"),
		logrus.WithOutFormat(log.JsonFormat),
		logrus.WithFileCutRule(log.CutByHour),
		logrus.WithCallerFullPath(true),
		logrus.WithFileClassifyStorage(true),
		logrus.WithOutStackLevel(log.ErrorLevel),
	)

	l.Warn(`log: warn`)
	l.Error(`log: error`)
}
