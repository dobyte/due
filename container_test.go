/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/11 1:01 下午
 * @Desc: TODO
 */

package due_test

import (
	"testing"

	"github.com/dobyte/due"
)

func TestNewContainer(t *testing.T) {
	c := due.NewContainer()

	c.Serve()
}
