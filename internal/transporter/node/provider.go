package node

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
)

type Provider interface {
	// Trigger triggers an event.
	Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error
	// Deliver delivers a message.
	Deliver(ctx context.Context, gid, nid string, cid, uid int64, buf buffer.Buffer) error
	// GetState returns the state.
	GetState() (cluster.State, error)
	// SetState sets the state.
	SetState(state cluster.State) error
}
