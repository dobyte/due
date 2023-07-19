/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/3 3:54 下午
 * @Desc: TODO
 */

package utils_test

import (
	"github.com/dobyte/due/log/zap/v2/internal/utils"
	"testing"
)

func TestAddslashes(t *testing.T) {
	str1 := "abc\\mas"
	t.Log(str1)
	t.Log(utils.Addslashes(str1))

	str2 := "abc\"mas"
	t.Log(str2)
	t.Log(utils.Addslashes(str2))

	str3 := "abc'mas"
	t.Log(str3)
	t.Log(utils.Addslashes(str3))

	str4 := "abc\nmas"
	t.Log(str4)
	t.Log(utils.Addslashes(str4))

	str5 := "abc\tmas"
	t.Log(str5)
	t.Log(utils.Addslashes(str5))
}
