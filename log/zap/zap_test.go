/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 5:08 下午
 * @Desc: TODO
 */

package zap_test

import (
	"testing"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/zap"
)

func TestNewLogger(t *testing.T) {
	l := zap.NewLogger(
		zap.WithOutFile("./log/log.log"),
		zap.WithOutLevel(log.DebugLevel),
	)

	l.Debug()

	//list := strings.Split("a.b.log" , ".")
	//
	//switch len(list) {
	//case 0:
	//
	//}
	//
	//if len(list)  2 {
	//
	//}
	//
	//
	//fmt.Println(strings.Split("a.b.log" , "."))
	//fmt.Println(strings.Split("a.b.log" , "." , 2))
}
