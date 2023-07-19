/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 5:08 下午
 * @Desc: TODO
 */

package zap_test

import (
	"github.com/dobyte/due/log/zap/v2"
	"testing"
)

var logger = zap.NewLogger()

func TestNewLogger(t *testing.T) {
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Error("error")
	logger.Fatal("fatal")
	logger.Panic("panic")
}
