package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/session"
)

// Peer is the cluster client contract shared by local and remote delivery.
type Peer interface {
	Bind(ctx context.Context, cid, uid int64) error
	Unbind(ctx context.Context, uid int64) error
	GetIP(ctx context.Context, kind session.Kind, target int64) (string, error)
	Stat(ctx context.Context, kind session.Kind) (int64, error)
	IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error)
	Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error
	Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer, ack bool) error
	Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer, ack bool) (int64, error)
	Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer, ack bool) (int64, error)
	Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer, ack bool) (int64, error)
	Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error
	Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error
	GetState(ctx context.Context) (cluster.State, error)
	SetState(ctx context.Context, state cluster.State) error
}

var _ Peer = (*Client)(nil)
var _ Peer = (*localClient)(nil)
