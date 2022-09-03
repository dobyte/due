/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/3 2:57 上午
 * @Desc: TODO
 */

package log_test

import (
	"testing"

	"github.com/dobyte/due/log"
)

func TestNewWriter(t *testing.T) {
	_, _ = log.NewWriter(log.WriterOptions{
		Level: log.InfoLevel,
	})
}
