/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/13 1:55 上午
 * @Desc: TODO
 */

package log_test

import (
	"testing"

	"github.com/dobyte/due/log"
)

func TestNewLogger(t *testing.T) {
	logger := log.NewLogger(
		log.WithOutFile("./log/due.log"),
		log.WithOutFormat(log.TextFormat),
		log.WithStackLevel(log.ErrorLevel),
		log.WithFileMaxSize(100*1024*1024),
		log.WithEnableLeveledStorage(false),
		//log.WithOutFile("./logs/due.log"),
		//log.WithOutLevel(log.InfoLevel),
		//log.WithOutFormat(log.TextFormat),
		//log.WithStackLevel(log.ErrorLevel),
		//log.WithFileMaxAge(100*1024*1024),
		//log.WithTimestampFormat("2006/01/02 15:04:05.000000"),
		//log.WithFileCutRule(log.CutByDay),
	)

	logger.Warn("aaa", "bbb")
}
