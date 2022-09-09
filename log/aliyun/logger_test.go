/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 12:03 下午
 * @Desc: TODO
 */

package aliyun_test

import (
	"testing"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/aliyun"
)

func TestNewLogger(t *testing.T) {
	l := aliyun.NewLogger(
		aliyun.WithProject("due-test"),
		aliyun.WithLogstore("app"),
		aliyun.WithEndpoint(""),
		aliyun.WithAccessKeyID(""),
		aliyun.WithAccessKeySecret(""),
		aliyun.WithStackLevel(log.InfoLevel),
	)
	defer l.Close()

	l.Info("info")
}
