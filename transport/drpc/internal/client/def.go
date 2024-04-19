package client

import (
	"context"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
)

type chWrite struct {
	ctx  context.Context
	seq  uint64
	buf  packet.IBuffer
	data []byte
	call *Call
}
