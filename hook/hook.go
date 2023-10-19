package hook

import (
	"context"
	"github.com/symsimmy/due/packet"
)

type ReceiveHook func(ctx context.Context, cid, uid int64, message *packet.Message)
