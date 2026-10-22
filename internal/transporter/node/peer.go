package node

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
)

// Peer is the cluster client contract shared by local and remote delivery.
type Peer interface {
	Trigger(ctx context.Context, event cluster.Event, cid, uid int64) error
	Deliver(ctx context.Context, cid, uid int64, buf buffer.Buffer) error
	GetState(ctx context.Context) (cluster.State, error)
	SetState(ctx context.Context, state cluster.State) error
}

var _ Peer = (*Client)(nil)
var _ Peer = (*localClient)(nil)
