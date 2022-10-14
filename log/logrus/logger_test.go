/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/30 5:36 下午
 * @Desc: TODO
 */

package logrus_test

import (
	"github.com/dobyte/due/log"
	"testing"

	"github.com/dobyte/due/log/logrus"
)

func TestNewLogger(t *testing.T) {
	l := logrus.NewLogger(
		logrus.WithFile("./log/due.log"),
		logrus.WithFormat(log.JsonFormat),
		logrus.WithStackLevel(log.ErrorLevel),
		logrus.WithFileCutRule(log.CutByHour),
		logrus.WithCallerFullPath(true),
		logrus.WithClassifiedStorage(true),
	)

	l.Warn(`log: warn`)
	l.Error(`log: error`)
}
