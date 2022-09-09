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

func TestNewLogger(t *testing.T) {
	l := aliyun.NewLogger(
		aliyun.WithProject("due-test"),
		aliyun.WithLogstore("app"),
		aliyun.WithEndpoint("cn-guangzhou.log.aliyuncs.com"),
		aliyun.WithAccessKeyID("LTAI5tKwurmJ2AJi6EYFEga8"),
		aliyun.WithAccessKeySecret("hXwR1rtW4DcByOQ4LgR1rpfk7JcR8E"),
	)
	defer l.Close()

	l.Debug("debug")
}
