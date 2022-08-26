/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/13 1:55 上午
 * @Desc: TODO
 */

package log_test

import (
	"log"
	"testing"

	log2 "github.com/dobyte/due/log"
)

func TestNewLogger(t *testing.T) {
	logger := log2.NewLogger(
		log2.WithWriter(log.Writer()),
		log2.WithFlag(log.Ldate|log.Lmicroseconds),
	)

	logger.Info("aaa", "bbb")
}
