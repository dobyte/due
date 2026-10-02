package gate

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/session"
)

type Provider interface {
	// Bind binds the relationship between the user and the gate.
	Bind(ctx context.Context, cid, uid int64) error
	// Unbind unbinds the relationship between the user and the gate.
	Unbind(ctx context.Context, uid int64) error
	// GetIP returns the client IP address.
	GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, err error)
	// IsOnline reports whether the target is online.
	IsOnline(ctx context.Context, kind session.Kind, target int64) (isOnline bool, err error)
	// Stat returns the total number of sessions.
	Stat(ctx context.Context, kind session.Kind) (total int64, err error)
	// Disconnect disconnects the target session.
	Disconnect(ctx context.Context, kind session.Kind, target int64, force bool) error
	// Push sends a message.
	Push(ctx context.Context, kind session.Kind, target int64, disconnect bool, buf buffer.Buffer) error
	// Multicast pushes a multicast message.
	Multicast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, buf buffer.Buffer) (total int64, err error)
	// Broadcast pushes a broadcast message.
	Broadcast(ctx context.Context, kind session.Kind, disconnect bool, buf buffer.Buffer) (total int64, err error)
	// Publish publishes a channel message.
	Publish(ctx context.Context, channel string, disconnect bool, buf buffer.Buffer) (total int64, err error)
	// Subscribe subscribes to channels.
	Subscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error
	// Unsubscribe unsubscribes from channels.
	Unsubscribe(ctx context.Context, kind session.Kind, targets []int64, channel string) error
	// GetState returns the state.
	GetState() (cluster.State, error)
	// SetState sets the state.
	SetState(state cluster.State) error
}
