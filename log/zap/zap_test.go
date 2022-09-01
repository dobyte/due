/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 5:08 下午
 * @Desc: TODO
 */

package zap_test

import (
	"testing"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/zap"
)

func TestNewLogger(t *testing.T) {
	l := zap.NewLogger(
		zap.WithOutLevel(log.DebugLevel),
	)

	l.Debug()
}
