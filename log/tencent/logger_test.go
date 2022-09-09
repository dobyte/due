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
		tencent.WithAccessKeyID("AKIDe8QUJtJpCRaWExSs7B1d3GEzWqwMbcOw"),
		tencent.WithAccessKeySecret("NVP4UgiV3NT3PXQ6XjhRwCE10kGXaOxJ"),
		tencent.WithTopicID("ff3fd9ba-360e-4cb9-8066-229c2290b213"),
		tencent.WithStackLevel(log.InfoLevel),
	)
	defer l.Close()

	l.Info("info")
}
