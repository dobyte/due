/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 5:08 下午
 * @Desc: TODO
 */

package zap_test

import (
	"github.com/dobyte/due/log/zap/v2"
	"github.com/dobyte/due/v2/log"
	"testing"
)

var logger = zap.NewLogger(
	zap.WithStackLevel(log.DebugLevel),
	zap.WithFormat(log.JsonFormat),
)

func TestNewLogger(t *testing.T) {
	//logger.Print(log.ErrorLevel, "print")
	//logger.Info("info")
	//logger.Warn("warn")
	//logger.Error("error")
	//logger.Error("error")
	//logger.Fatal("fatal")
	logger.Panic("panic")
}
