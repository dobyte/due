/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 6:30 下午
 * @Desc: TODO
 */

package tencent_test

import (
	"github.com/dobyte/due/log/tencent/v2"
	"testing"
)

var logger = tencent.NewLogger()

func TestNewLogger(t *testing.T) {
	defer logger.Close()

	logger.Error("error")
}
