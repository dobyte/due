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
	// 注意(非强制): zap.WithFormat(log.JsonFormat) 设置为 log.TextFormat 时, 新增 zap.WithCallerSkip(1) 选项配置, 控制台
	// 才会打印调用者堆栈信息
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
