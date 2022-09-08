/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/8 3:36 上午
 * @Desc: TODO
 */

package log_test

import (
	"testing"

	"github.com/dobyte/due/log"
)

func TestLogger(t *testing.T) {
	//log.Debug("debug")
	//log.Info("info")
	//log.Warn("warn")
	//log.Error("error")
	//log.Fatal("fatal")
	log.Panic("panic")
}
