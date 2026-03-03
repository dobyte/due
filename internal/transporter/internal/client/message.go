package client

import (
	"sync/atomic"

	"github.com/dobyte/due/v2/core/buffer"
)

const (
	statePending  = 0 // 待发送
	stateSent     = 1 // 已发送
	stateCanceled = 2 // 已取消
)

type message struct {
	seq   uint64               // 序列号
	buf   *buffer.NocopyBuffer // 数据buffer
	call  chan buffer.Buffer   // 回调数据
	state atomic.Int32         // 消息状态
}
