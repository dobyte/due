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
		log.WithOutFormat(log.JsonFormat),
		log.WithOutStackLevel(log.InfoLevel),
		log.WithFileClassifyStorage(true),
	)

	logger.Warn("aaa", "bbb")
}
