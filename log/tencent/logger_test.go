/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/9 6:30 下午
 * @Desc: TODO
 */

package tencent_test

import (
	"testing"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/tencent"
)

func TestNewLogger(t *testing.T) {
	l := tencent.NewLogger(
		tencent.WithEndpoint("ap-guangzhou.cls.tencentcs.com"),
		tencent.WithAccessKeyID(""),
		tencent.WithAccessKeySecret(""),
		tencent.WithTopicID(""),
		tencent.WithStackLevel(log.InfoLevel),
	)
	defer l.Close()

	l.Info("info")
}
