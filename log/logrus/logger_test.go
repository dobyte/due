/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/30 5:36 下午
 * @Desc: TODO
 */

package logrus_test

import (
	"testing"

	"github.com/dobyte/due/log/logrus"
)

var logger *logrus.Logger

func init() {
	logger = logrus.NewLogger()
}

func TestNewLogger(t *testing.T) {
	logger.Warn(`log: warn`)
	logger.Error(`log: error`)
}
