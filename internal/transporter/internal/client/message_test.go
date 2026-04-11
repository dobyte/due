package client_test

import (
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/dobyte/due/v2/core/buffer"
)

type message struct {
	seq   uint64               // 序列号
	buf   *buffer.NocopyBuffer // 数据buffer
	call  chan buffer.Buffer   // 回调数据
	state atomic.Int32         // 消息状态
}

func TestMessage(t *testing.T) {
	t.Log(unsafe.Sizeof(message{}))
}
