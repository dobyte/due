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
		logrus.WithOutFile("./log.txt"),
		logrus.WithOutFormat(log.JsonFormat),
		logrus.WithFileCutRule(log.DayCutRule),
		logrus.WithCallerFullPath(true),
	)

	for i := 0; i < 1000; i++ {
		l.Warn(`"hello world"`)
	}

	//l.Error("hello world")
	//l.Fatal("hello world")
}
