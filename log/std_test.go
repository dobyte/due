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
	"github.com/dobyte/due/mode"
)

func TestNewLogger(t *testing.T) {
	mode.SetMode(mode.TestMode)

	logger := log.NewLogger(
		log.WithOutFile("./log/due.log"),
		log.WithOutFormat(log.TextFormat),
		log.WithStackLevel(log.InfoLevel),
		log.WithEnableLeveledStorage(true),
	)

	logger.Warn("aaa", "bbb")
}
