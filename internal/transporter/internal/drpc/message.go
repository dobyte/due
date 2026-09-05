package drpc

import (
	"github.com/dobyte/due/v2/core/buffer"
)

// message 发送消息，仅承载待发送的数据缓冲
type message struct {
	buf *buffer.NocopyBuffer // 数据buffer
}
