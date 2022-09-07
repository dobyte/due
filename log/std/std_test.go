/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/13 1:55 上午
 * @Desc: TODO
 */

package std_test

import (
	"testing"

	"github.com/dobyte/due/log/std"
	"github.com/dobyte/due/mode"
)

func TestNewLogger(t *testing.T) {
	mode.SetMode(mode.TestMode)

	logger := std.NewLogger(
		std.WithOutFile("./log/due.log"),
		std.WithOutFormat(std.TextFormat),
		std.WithStackLevel(std.InfoLevel),
		std.WithEnableLeveledStorage(true),
	)

	logger.Warn("aaa", "bbb")
}
