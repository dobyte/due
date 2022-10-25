/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 12:03 下午
 * @Desc: TODO
 */

package aliyun_test

import (
	"testing"

	"github.com/dobyte/due/log/aliyun"
)

var logger *aliyun.Logger

func init() {
	logger = aliyun.NewLogger()
}

func TestNewLogger(t *testing.T) {
	defer logger.Close()

	logger.Info("info")
}
