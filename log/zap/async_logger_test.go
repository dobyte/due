/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 5:08 下午
 * @Desc: TODO
 */

package zap_test

import (
	"github.com/symsimmy/due/log/zap"
	"testing"
	"time"
)

var asyncLogger *zap.AsyncLogger

func init() {
	asyncLogger = zap.NewAsyncLogger()
}

func BenchmarkAsyncLogger(b *testing.B) {
	for n := 0; n < b.N; n++ {
		asyncLogger.Info("info")
		asyncLogger.Warn("warn")
		asyncLogger.Error("error")
	}
}

func TestAsyncLogger(t *testing.T) {
	timeTick := time.Tick(1 * time.Second)
	timeAfter := time.After(180 * time.Second)
	for {
		select {
		case <-timeTick:
			asyncLogger.Info("info")
		case <-timeAfter:
			return
		}
	}
}
