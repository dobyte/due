/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 5:08 下午
 * @Desc: TODO
 */

package zap_test

import (
	"testing"

	"github.com/dobyte/due/log/zap"
)

func TestNewLogger(t *testing.T) {
	//l := zap.NewLogger(
	//	zap.WithFile("./log/due.log"),
	//	zap.WithLevel(log.WarnLevel),
	//	zap.WithFormat(log.JsonFormat),
	//	zap.WithStackLevel(log.WarnLevel),
	//	zap.WithClassifiedStorage(true),
	//)

	l := zap.NewLogger()

	//l.Info("info")
	//l.Warn("warn")
	l.Error("error")
	//l.Error("error")
	//l.Fatal("fatal")
	//l.Panic("panic")
}
